package service

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"sort"
	"strings"
	"time"

	"xraytool/internal/model"

	"github.com/skip2/go-qrcode"
	"github.com/xuri/excelize/v2"
)

type XLSXExportOptions struct {
	Count            int
	Shuffle          bool
	IncludeRawSocks5 bool
}

type xlsxExportRow struct {
	Protocol     string
	GroupHeadID  uint
	Customer     string
	CustomerCode string
	CountryCode  string
	CycleTag     string
	DurationDays int
	ExpiresAt    time.Time
	Link         string
	RawSocks5    string
	QRCodeData   []byte
}

func (s *OrderService) BatchExportXLSX(orderIDs []uint) ([]byte, string, error) {
	body, filename, contentType, err := s.BatchExportArtifact(orderIDs, XLSXExportOptions{Shuffle: true})
	if err != nil {
		return nil, "", err
	}
	if contentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		return nil, "", errors.New("batch export contains multiple protocols, use zip artifact endpoint")
	}
	return body, filename, nil
}

func (s *OrderService) ExportOrderXLSX(orderID uint, opts ExportOrderOptions) ([]byte, string, error) {
	rows, err := s.collectXLSXRows([]uint{orderID}, XLSXExportOptions{Count: opts.Count, Shuffle: opts.Shuffle})
	if err != nil {
		return nil, "", err
	}
	if len(rows) == 0 {
		return nil, "", errors.New("no active items")
	}
	protocol := rows[0].Protocol
	body, err := buildOrdersXLSX(rows, protocol, false)
	if err != nil {
		return nil, "", err
	}
	filename := exportArtifactName(rows, protocol) + ".xlsx"
	return body, filename, nil
}

func (s *OrderService) ExportOrderArtifact(orderID uint, opts ExportOrderOptions, includeRaw bool) ([]byte, string, string, error) {
	rows, err := s.collectXLSXRows([]uint{orderID}, XLSXExportOptions{Count: opts.Count, Shuffle: opts.Shuffle, IncludeRawSocks5: includeRaw})
	if err != nil {
		return nil, "", "", err
	}
	if len(rows) == 0 {
		return nil, "", "", errors.New("no active items")
	}
	protocol := rows[0].Protocol
	body, err := buildOrdersXLSX(rows, protocol, includeRaw)
	if err != nil {
		return nil, "", "", err
	}
	filename := exportArtifactName(rows, protocol) + ".xlsx"
	return body, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

func (s *OrderService) BatchExportArtifact(orderIDs []uint, opts XLSXExportOptions) ([]byte, string, string, error) {
	rows, err := s.collectXLSXRows(orderIDs, opts)
	if err != nil {
		return nil, "", "", err
	}
	if len(rows) == 0 {
		return nil, "", "", errors.New("no active items")
	}
	groups := map[string][]xlsxExportRow{}
	for _, row := range rows {
		key := fmt.Sprintf("%s|%d", row.Protocol, row.GroupHeadID)
		groups[key] = append(groups[key], row)
	}
	if len(groups) == 1 {
		for _, one := range groups {
			body, err := buildOrdersXLSX(one, one[0].Protocol, opts.IncludeRawSocks5)
			if err != nil {
				return nil, "", "", err
			}
			filename := exportArtifactName(one, one[0].Protocol) + ".xlsx"
			return body, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
		}
	}
	zipBody, err := buildExportZip(groups, opts.IncludeRawSocks5)
	if err != nil {
		return nil, "", "", err
	}
	filename := fmt.Sprintf("batch-export-%s.zip", time.Now().Format("20060102-150405"))
	return zipBody, filename, "application/zip", nil
}

func (s *OrderService) collectXLSXRows(orderIDs []uint, opts XLSXExportOptions) ([]xlsxExportRow, error) {
	orders, err := s.expandOrdersForExport(orderIDs)
	if err != nil {
		return nil, err
	}
	linkSettings := s.loadExportLinkSettings()
	itemIDs := make([]uint, 0)
	for _, order := range orders {
		for _, item := range order.Items {
			itemIDs = append(itemIDs, item.ID)
		}
	}
	egressByItem := map[uint]model.DedicatedEgress{}
	if len(itemIDs) > 0 {
		rows := []model.DedicatedEgress{}
		if err := s.db.Where("order_item_id in ?", itemIDs).Find(&rows).Error; err == nil {
			for _, row := range rows {
				egressByItem[row.OrderItemID] = row
			}
		}
	}

	result := make([]xlsxExportRow, 0)
	now := time.Now()
	for _, order := range orders {
		if order.Status != model.OrderStatusActive || !order.ExpiresAt.After(now) {
			continue
		}
		protocol := exportOrderProtocol(order)
		for _, item := range order.Items {
			if item.Status != model.OrderItemStatusActive {
				continue
			}
			link := buildOrderItemLinkByProtocol(order, item, protocol, linkSettings)
			if strings.TrimSpace(link) == "" {
				continue
			}
			qr, err := buildSingleProtocolQRCodeImage(link)
			if err != nil {
				return nil, err
			}
			egress := egressByItem[item.ID]
			country := strings.ToLower(strings.TrimSpace(egress.CountryCode))
			if country == "" {
				country = "xx"
			}
			rawSocks := fmt.Sprintf("%s:%d:%s:%s", item.ForwardAddress, item.ForwardPort, item.ForwardUsername, item.ForwardPassword)
			if strings.TrimSpace(item.ForwardAddress) == "" || item.ForwardPort <= 0 {
				rawSocks = ""
			}
			result = append(result, xlsxExportRow{
				Protocol:     protocol,
				GroupHeadID:  groupHeadID(order),
				Customer:     strings.TrimSpace(order.Customer.Name),
				CustomerCode: strings.TrimSpace(order.Customer.Code),
				CountryCode:  country,
				CycleTag:     cycleTag(order, now),
				DurationDays: cycleDays(order, now),
				ExpiresAt:    order.ExpiresAt,
				Link:         link,
				RawSocks5:    rawSocks,
				QRCodeData:   qr,
			})
		}
	}
	if len(result) == 0 {
		return nil, errors.New("no active items")
	}
	if opts.Shuffle {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(result), func(i, j int) {
			result[i], result[j] = result[j], result[i]
		})
	}
	if opts.Count > 0 {
		if opts.Count > len(result) {
			return nil, fmt.Errorf("extract count %d exceeds active items %d", opts.Count, len(result))
		}
		result = result[:opts.Count]
	}
	return result, nil
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
		if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").Preload("Items").First(&order, id).Error; err != nil {
			return nil, err
		}
		if order.IsGroupHead {
			children := []model.Order{}
			if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").Preload("Items").Where("parent_order_id = ?", order.ID).Order("sequence_no asc, id asc").Find(&children).Error; err != nil {
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

func buildExportZip(groups map[string][]xlsxExportRow, includeRaw bool) ([]byte, error) {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buf := bytes.NewBuffer(nil)
	zipWriter := zip.NewWriter(buf)
	for _, key := range keys {
		rows := groups[key]
		if len(rows) == 0 {
			continue
		}
		protocol := rows[0].Protocol
		xlsxBody, err := buildOrdersXLSX(rows, protocol, includeRaw)
		if err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
		name := exportArtifactName(rows, protocol) + ".xlsx"
		file, err := zipWriter.Create(name)
		if err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
		if _, err := file.Write(xlsxBody); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func exportArtifactName(rows []xlsxExportRow, protocol string) string {
	if len(rows) == 0 {
		return "export"
	}
	row := rows[0]
	customer := strings.TrimSpace(row.Customer)
	if customer == "" {
		customer = strings.TrimSpace(row.CustomerCode)
	}
	if customer == "" {
		customer = "customer"
	}
	orderNo := fmt.Sprintf("OD%s%s", time.Now().Format("060102"), randomDigits(6))
	protocolLabel := exportProtocolLabel(protocol)
	countryPart := exportCountryStatLabel(rows)
	days := row.DurationDays
	if days <= 0 {
		days = 30
	}
	name := fmt.Sprintf("%s-%s-[%s]-[%s]-%d天", customer, orderNo, protocolLabel, countryPart, days)
	return sanitizeFilename(name)
}

func sanitizeFilename(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "export"
	}
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
	)
	out := strings.TrimSpace(replacer.Replace(raw))
	if out == "" {
		return "export"
	}
	return out
}

func randomDigits(n int) string {
	if n <= 0 {
		return ""
	}
	const digits = "0123456789"
	b := make([]byte, n)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < n; i++ {
		b[i] = digits[r.Intn(len(digits))]
	}
	return string(b)
}

func exportProtocolLabel(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case model.DedicatedFeatureVmess:
		return "Vmess"
	case model.DedicatedFeatureVless:
		return "Vless"
	case model.DedicatedFeatureShadowsocks:
		return "Shadowsocks"
	default:
		return "Socks5"
	}
}

func exportCountryStatLabel(rows []xlsxExportRow) string {
	if len(rows) == 0 {
		return "0条未知"
	}
	counts := map[string]int{}
	for _, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.CountryCode))
		if key == "" {
			key = "xx"
		}
		counts[key]++
	}
	type pair struct {
		Country string
		Count   int
	}
	pairs := make([]pair, 0, len(counts))
	for country, count := range counts {
		pairs = append(pairs, pair{Country: country, Count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count == pairs[j].Count {
			return pairs[i].Country < pairs[j].Country
		}
		return pairs[i].Count > pairs[j].Count
	})
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, fmt.Sprintf("%d条%s", p.Count, countryNameCN(p.Country)))
	}
	return strings.Join(parts, "|")
}

func countryNameCN(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" || code == "xx" {
		return "未知"
	}
	known := map[string]string{
		"us": "美国",
		"mx": "墨西哥",
		"ca": "加拿大",
		"jp": "日本",
		"sg": "新加坡",
		"kr": "韩国",
		"gb": "英国",
		"de": "德国",
		"fr": "法国",
		"nl": "荷兰",
		"hk": "香港",
		"tw": "台湾",
	}
	if name, ok := known[code]; ok {
		return name
	}
	return strings.ToUpper(code)
}

func cycleTag(order model.Order, now time.Time) string {
	days := cycleDays(order, now)
	return fmt.Sprintf("D%d", days)
}

func cycleDays(order model.Order, now time.Time) int {
	base := order.StartsAt
	if base.IsZero() || base.After(order.ExpiresAt) {
		base = now
	}
	days := int(order.ExpiresAt.Sub(base).Hours() / 24)
	if days <= 0 {
		days = 1
	}
	return days
}

func groupHeadID(order model.Order) uint {
	if order.ParentOrderID != nil && *order.ParentOrderID > 0 {
		return *order.ParentOrderID
	}
	if order.GroupID > 0 {
		return order.GroupID
	}
	return order.ID
}

func exportOrderProtocol(order model.Order) string {
	protocol := strings.ToLower(strings.TrimSpace(order.DedicatedProtocol))
	if protocol == "" {
		protocol = model.DedicatedFeatureMixed
	}
	if order.Mode != model.OrderModeDedicated {
		return model.DedicatedFeatureMixed
	}
	return protocol
}

type exportLinkSettings struct {
	VlessSecurity string
	VlessSNI      string
	VlessType     string
	VlessPath     string
	VlessHost     string
}

func (s *OrderService) loadExportLinkSettings() exportLinkSettings {
	out := exportLinkSettings{
		VlessSecurity: "tls",
		VlessType:     "tcp",
	}
	rows := []model.Setting{}
	if err := s.db.Where("key in ?", []string{"dedicated_vless_security", "dedicated_vless_sni", "dedicated_vless_type", "dedicated_vless_path", "dedicated_vless_host"}).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		v := strings.TrimSpace(row.Value)
		switch strings.TrimSpace(row.Key) {
		case "dedicated_vless_security":
			if v != "" {
				out.VlessSecurity = v
			}
		case "dedicated_vless_sni":
			out.VlessSNI = v
		case "dedicated_vless_type":
			if v != "" {
				out.VlessType = v
			}
		case "dedicated_vless_path":
			out.VlessPath = v
		case "dedicated_vless_host":
			out.VlessHost = v
		}
	}
	return out
}

func buildOrderItemLinkByProtocol(order model.Order, item model.OrderItem, protocol string, cfg exportLinkSettings) string {
	if order.Mode != model.OrderModeDedicated {
		auth := base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s@%s:%d", item.Username, item.Password, item.IP, item.Port)))
		return fmt.Sprintf("socks://%s?method=auto", auth)
	}
	host := ""
	port := order.Port
	if order.DedicatedIngress != nil {
		host = strings.TrimSpace(order.DedicatedIngress.Domain)
		if order.DedicatedIngress.IngressPort > 0 {
			port = order.DedicatedIngress.IngressPort
		}
	}
	if host == "" && order.DedicatedEntry != nil {
		host = strings.TrimSpace(order.DedicatedEntry.Domain)
	}
	if host == "" {
		host = strings.TrimSpace(item.IP)
	}
	remark := strings.TrimSpace(order.Name)
	if remark == "" {
		remark = fmt.Sprintf("order-%d", order.ID)
	}
	uuid := strings.TrimSpace(item.VmessUUID)
	if uuid == "" {
		uuid = randomUUID()
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case model.DedicatedFeatureVmess:
		if port <= 0 {
			return ""
		}
		raw := fmt.Sprintf(`{"v":"2","ps":"%s","add":"%s","port":"%d","id":"%s","aid":"0","net":"tcp","type":"none","host":"","path":"","tls":""}`,
			remark, host, port, uuid,
		)
		return "vmess://" + base64.StdEncoding.EncodeToString([]byte(raw))
	case model.DedicatedFeatureVless:
		if port <= 0 {
			return ""
		}
		security := strings.TrimSpace(cfg.VlessSecurity)
		if security == "" {
			security = "tls"
		}
		vType := strings.TrimSpace(cfg.VlessType)
		if vType == "" {
			vType = "tcp"
		}
		params := url.Values{}
		params.Set("encryption", "none")
		params.Set("security", security)
		params.Set("type", vType)
		sni := strings.TrimSpace(cfg.VlessSNI)
		if strings.EqualFold(security, "tls") && sni == "" {
			sni = host
		}
		if sni != "" {
			params.Set("sni", sni)
		}
		if strings.TrimSpace(cfg.VlessPath) != "" {
			params.Set("path", strings.TrimSpace(cfg.VlessPath))
		}
		if strings.TrimSpace(cfg.VlessHost) != "" {
			params.Set("host", strings.TrimSpace(cfg.VlessHost))
		}
		return fmt.Sprintf("vless://%s@%s:%d?%s#%s", uuid, host, port, params.Encode(), url.QueryEscape(remark))
	case model.DedicatedFeatureShadowsocks:
		if port <= 0 {
			return ""
		}
		raw := fmt.Sprintf("%s:%s@%s:%d", DedicatedShadowsocksMethod, item.Password, host, port)
		return fmt.Sprintf("ss://%s", base64.RawStdEncoding.EncodeToString([]byte(raw)))
	default:
		if port <= 0 {
			return ""
		}
		auth := base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s@%s:%d", item.Username, item.Password, host, port)))
		return fmt.Sprintf("socks://%s?method=auto", auth)
	}
}

func buildSingleProtocolQRCodeImage(link string) ([]byte, error) {
	if strings.TrimSpace(link) == "" {
		return nil, nil
	}
	return qrcode.Encode(link, qrcode.Medium, 256)
}

func buildOrdersXLSX(rows []xlsxExportRow, protocol string, includeRaw bool) ([]byte, error) {
	const qrRowHeight = 128.0
	const qrColWidth = 23.0
	const qrScale = 0.43
	const qrOffset = 6

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"链接", "图片", "到期时间"}
	if includeRaw {
		headers = append(headers, "原始Socks5")
	}
	for i, title := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, title)
	}
	wrapStyle, _ := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	for idx, row := range rows {
		r := idx + 2
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r), row.Link)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", r), row.ExpiresAt.Format("2006-01-02 15:04:05"))
		if includeRaw {
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", r), row.RawSocks5)
		}
		_ = f.SetRowHeight(sheet, r, qrRowHeight)
		if len(row.QRCodeData) > 0 {
			_ = f.AddPictureFromBytes(sheet, fmt.Sprintf("B%d", r), &excelize.Picture{
				Extension: ".png",
				File:      row.QRCodeData,
				Format: &excelize.GraphicOptions{
					OffsetX:         qrOffset,
					OffsetY:         qrOffset,
					ScaleX:          qrScale,
					ScaleY:          qrScale,
					LockAspectRatio: true,
					Positioning:     "oneCell",
				},
			})
		}
	}
	_ = f.SetColWidth(sheet, "A", "A", 64)
	_ = f.SetColWidth(sheet, "B", "B", qrColWidth)
	_ = f.SetColWidth(sheet, "C", "C", 22)
	if includeRaw {
		_ = f.SetColWidth(sheet, "D", "D", 40)
	}
	endCol := "C"
	if includeRaw {
		endCol = "D"
	}
	_ = f.SetCellStyle(sheet, "A1", endCol+"1", wrapStyle)
	if len(rows) > 0 {
		_ = f.SetCellStyle(sheet, "A2", fmt.Sprintf("%s%d", endCol, len(rows)+1), wrapStyle)
	}
	_ = f.SetCellValue(sheet, "F1", strings.ToUpper(protocol))
	meta := "二维码参数表"
	if _, err := f.NewSheet(meta); err == nil {
		_ = f.SetCellValue(meta, "A1", "客户端")
		_ = f.SetCellValue(meta, "B1", "行高(pt)")
		_ = f.SetCellValue(meta, "C1", "列宽")
		_ = f.SetCellValue(meta, "D1", "缩放")
		_ = f.SetCellValue(meta, "E1", "备注")
		_ = f.SetCellValue(meta, "A2", "Excel")
		_ = f.SetCellValue(meta, "B2", 124)
		_ = f.SetCellValue(meta, "C2", 22.5)
		_ = f.SetCellValue(meta, "D2", 0.42)
		_ = f.SetCellValue(meta, "E2", "二维码清晰且不拉伸")
		_ = f.SetCellValue(meta, "A3", "WPS")
		_ = f.SetCellValue(meta, "B3", 128)
		_ = f.SetCellValue(meta, "C3", 23)
		_ = f.SetCellValue(meta, "D3", 0.43)
		_ = f.SetCellValue(meta, "E3", "边界稳定，避免覆盖文本")
		_ = f.SetCellValue(meta, "A4", "当前采用")
		_ = f.SetCellValue(meta, "B4", qrRowHeight)
		_ = f.SetCellValue(meta, "C4", qrColWidth)
		_ = f.SetCellValue(meta, "D4", qrScale)
		_ = f.SetCellValue(meta, "E4", "兼容 Excel/WPS")
		_ = f.SetColWidth(meta, "A", "E", 22)
	}
	body, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}
