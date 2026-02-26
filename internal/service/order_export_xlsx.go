package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"net/url"
	"sort"
	"strings"
	"time"

	"xraytool/internal/model"

	"github.com/skip2/go-qrcode"
	"github.com/xuri/excelize/v2"
)

type xlsxExportRow struct {
	OrderID    uint
	Customer   string
	OrderName  string
	EntryName  string
	ExpiresAt  time.Time
	Socks5     string
	Vmess      string
	Vless      string
	SS         string
	QRCodeData []byte
}

func (s *OrderService) BatchExportXLSX(orderIDs []uint) ([]byte, string, error) {
	rows, err := s.collectXLSXRows(orderIDs, ExportOrderOptions{Shuffle: true})
	if err != nil {
		return nil, "", err
	}
	body, err := buildOrdersXLSX(rows)
	if err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("batch-orders-%s.xlsx", time.Now().Format("20060102-150405"))
	return body, name, nil
}

func (s *OrderService) ExportOrderXLSX(orderID uint, opts ExportOrderOptions) ([]byte, string, error) {
	rows, err := s.collectXLSXRows([]uint{orderID}, opts)
	if err != nil {
		return nil, "", err
	}
	body, err := buildOrdersXLSX(rows)
	if err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("order-%d-export.xlsx", orderID)
	return body, name, nil
}

func (s *OrderService) collectXLSXRows(orderIDs []uint, opts ExportOrderOptions) ([]xlsxExportRow, error) {
	orders, err := s.expandOrdersForExport(orderIDs)
	if err != nil {
		return nil, err
	}
	rows := make([]xlsxExportRow, 0)
	now := time.Now()
	for _, order := range orders {
		if order.Status != model.OrderStatusActive || !order.ExpiresAt.After(now) {
			continue
		}
		entryName := "-"
		if order.DedicatedEntry != nil {
			if strings.TrimSpace(order.DedicatedEntry.Name) != "" {
				entryName = strings.TrimSpace(order.DedicatedEntry.Name)
			} else {
				entryName = fmt.Sprintf("%s", strings.TrimSpace(order.DedicatedEntry.Domain))
			}
		}
		for _, item := range order.Items {
			if item.Status != model.OrderItemStatusActive {
				continue
			}
			socks, vmess, vless, ss := buildOrderItemLinks(order, item)
			qr, qrErr := buildProtocolQRCodeImage(socks, vmess, vless, ss)
			if qrErr != nil {
				return nil, qrErr
			}
			rows = append(rows, xlsxExportRow{
				OrderID:    order.ID,
				Customer:   strings.TrimSpace(order.Customer.Name),
				OrderName:  strings.TrimSpace(order.Name),
				EntryName:  entryName,
				ExpiresAt:  order.ExpiresAt,
				Socks5:     socks,
				Vmess:      vmess,
				Vless:      vless,
				SS:         ss,
				QRCodeData: qr,
			})
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("no active items")
	}
	if opts.Shuffle {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(rows), func(i, j int) {
			rows[i], rows[j] = rows[j], rows[i]
		})
	}
	if opts.Count > 0 {
		if opts.Count > len(rows) {
			return nil, fmt.Errorf("extract count %d exceeds active items %d", opts.Count, len(rows))
		}
		rows = rows[:opts.Count]
	}
	return rows, nil
}

func (s *OrderService) expandOrdersForExport(orderIDs []uint) ([]model.Order, error) {
	ids := uniqueUintIDs(orderIDs)
	if len(ids) == 0 {
		return nil, errors.New("order_ids is empty")
	}
	result := make([]model.Order, 0)
	seen := map[uint]struct{}{}
	for _, id := range ids {
		order := model.Order{}
		if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("Items").First(&order, id).Error; err != nil {
			return nil, err
		}
		if order.IsGroupHead {
			children := []model.Order{}
			if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("Items").Where("parent_order_id = ?", order.ID).Order("sequence_no asc, id asc").Find(&children).Error; err != nil {
				return nil, err
			}
			if len(children) == 0 {
				if _, ok := seen[order.ID]; !ok {
					seen[order.ID] = struct{}{}
					result = append(result, order)
				}
				continue
			}
			for _, child := range children {
				if _, ok := seen[child.ID]; ok {
					continue
				}
				seen[child.ID] = struct{}{}
				result = append(result, child)
			}
			continue
		}
		if _, ok := seen[order.ID]; ok {
			continue
		}
		seen[order.ID] = struct{}{}
		result = append(result, order)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func buildOrderItemLinks(order model.Order, item model.OrderItem) (string, string, string, string) {
	if order.Mode != model.OrderModeDedicated || order.DedicatedEntry == nil {
		return fmt.Sprintf("%s:%d:%s:%s", item.IP, item.Port, item.Username, item.Password), "", "", ""
	}
	entry := order.DedicatedEntry
	features := parseDedicatedFeatures(entry.Features)
	host := strings.TrimSpace(entry.Domain)
	if host == "" {
		host = strings.TrimSpace(item.IP)
	}
	remark := strings.TrimSpace(order.Name)
	if remark == "" {
		remark = fmt.Sprintf("order-%d", order.ID)
	}
	if strings.TrimSpace(order.Customer.Name) != "" {
		remark = fmt.Sprintf("%s-%s", strings.TrimSpace(order.Customer.Name), remark)
	}
	uuid := strings.TrimSpace(item.VmessUUID)
	if uuid == "" {
		uuid = randomUUID()
	}

	socks := ""
	if _, ok := features[model.DedicatedFeatureMixed]; ok && entry.MixedPort > 0 {
		socks = fmt.Sprintf("%s:%d:%s:%s", host, entry.MixedPort, item.Username, item.Password)
	}
	vmess := ""
	if _, ok := features[model.DedicatedFeatureVmess]; ok && entry.VmessPort > 0 {
		payload := map[string]string{
			"v":    "2",
			"ps":   remark,
			"add":  host,
			"port": fmt.Sprintf("%d", entry.VmessPort),
			"id":   uuid,
			"aid":  "0",
			"net":  "tcp",
			"type": "none",
			"host": "",
			"path": "",
			"tls":  "",
		}
		raw := fmt.Sprintf(`{"v":"%s","ps":"%s","add":"%s","port":"%s","id":"%s","aid":"%s","net":"%s","type":"%s","host":"%s","path":"%s","tls":"%s"}`,
			payload["v"], payload["ps"], payload["add"], payload["port"], payload["id"], payload["aid"], payload["net"], payload["type"], payload["host"], payload["path"], payload["tls"],
		)
		vmess = "vmess://" + base64.StdEncoding.EncodeToString([]byte(raw))
	}
	vless := ""
	if _, ok := features[model.DedicatedFeatureVless]; ok && entry.VlessPort > 0 {
		vless = fmt.Sprintf("vless://%s@%s:%d?encryption=none&security=none&type=tcp#%s", uuid, host, entry.VlessPort, url.QueryEscape(remark))
	}
	ss := ""
	if _, ok := features[model.DedicatedFeatureShadowsocks]; ok && entry.ShadowsocksPort > 0 {
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", DedicatedShadowsocksMethod, item.Password)))
		ss = fmt.Sprintf("ss://%s@%s:%d#%s", auth, host, entry.ShadowsocksPort, url.QueryEscape(remark))
	}
	return socks, vmess, vless, ss
}

func buildProtocolQRCodeImage(socks string, vmess string, vless string, ss string) ([]byte, error) {
	entries := []string{socks, vmess, vless, ss}
	cellSize := 168
	gap := 8
	canvasW := cellSize*2 + gap*3
	canvasH := cellSize*2 + gap*3
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	for idx, link := range entries {
		img := image.Image(image.NewRGBA(image.Rect(0, 0, cellSize, cellSize)))
		draw.Draw(img.(*image.RGBA), img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		if strings.TrimSpace(link) != "" {
			pngBody, err := qrcode.Encode(link, qrcode.Medium, cellSize)
			if err != nil {
				return nil, err
			}
			decoded, _, err := image.Decode(bytes.NewReader(pngBody))
			if err != nil {
				return nil, err
			}
			img = decoded
		}
		col := idx % 2
		row := idx / 2
		x := gap + col*(cellSize+gap)
		y := gap + row*(cellSize+gap)
		rect := image.Rect(x, y, x+cellSize, y+cellSize)
		draw.Draw(canvas, rect, img, image.Point{}, draw.Src)
	}
	out := bytes.NewBuffer(nil)
	if err := png.Encode(out, canvas); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func buildOrdersXLSX(rows []xlsxExportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"订单ID", "客户", "订单名", "入口", "Socks5", "Vmess", "Vless", "Shadowsocks", "图片", "到期时间"}
	for i, title := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, title)
	}
	wrapStyle, _ := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	for idx, row := range rows {
		r := idx + 2
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r), row.OrderID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", r), row.Customer)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", r), row.OrderName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", r), row.EntryName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", r), row.Socks5)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", r), row.Vmess)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", r), row.Vless)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", r), row.SS)
		_ = f.SetCellValue(sheet, fmt.Sprintf("J%d", r), row.ExpiresAt.Format("2006-01-02 15:04:05"))
		_ = f.SetRowHeight(sheet, r, 132)
		if len(row.QRCodeData) > 0 {
			_ = f.AddPictureFromBytes(sheet, fmt.Sprintf("I%d", r), &excelize.Picture{
				Extension: ".png",
				File:      row.QRCodeData,
				Format: &excelize.GraphicOptions{
					ScaleX: 0.72,
					ScaleY: 0.72,
				},
			})
		}
	}
	_ = f.SetColWidth(sheet, "A", "A", 10)
	_ = f.SetColWidth(sheet, "B", "D", 18)
	_ = f.SetColWidth(sheet, "E", "H", 44)
	_ = f.SetColWidth(sheet, "I", "I", 24)
	_ = f.SetColWidth(sheet, "J", "J", 22)
	_ = f.SetCellStyle(sheet, "A1", "J1", wrapStyle)
	if len(rows) > 0 {
		_ = f.SetCellStyle(sheet, "A2", fmt.Sprintf("J%d", len(rows)+1), wrapStyle)
	}
	body, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}
