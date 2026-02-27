package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/png"
	"net/url"
	"strings"
	"testing"
	"time"

	"xraytool/internal/model"

	"github.com/xuri/excelize/v2"
)

func TestDedicatedLinkTagUsesProbeCountryAndExitIP(t *testing.T) {
	tag := dedicatedLinkTag("us", "1.2.3.4")
	if tag != "美国-1.2.3.4" {
		t.Fatalf("unexpected tag: %q", tag)
	}
}

func TestBuildOrderItemLinksCarryChineseTag(t *testing.T) {
	order := model.Order{
		ID:   100,
		Mode: model.OrderModeDedicated,
		Port: 443,
		DedicatedIngress: &model.DedicatedIngress{
			Domain:      "line.example.com",
			IngressPort: 443,
		},
	}
	item := model.OrderItem{
		Username:  "user01",
		Password:  "pass01",
		VmessUUID: "11111111-2222-3333-4444-555555555555",
	}
	cfg := exportLinkSettings{VlessSecurity: "tls", VlessType: "tcp"}
	tag := "美国-1.2.3.4"
	encodedTag := url.QueryEscape(tag)

	socks := buildOrderItemLinkByProtocol(order, item, model.DedicatedFeatureMixed, cfg, tag)
	if !strings.Contains(socks, "#"+encodedTag) {
		t.Fatalf("socks link should contain encoded tag, got: %s", socks)
	}

	vless := buildOrderItemLinkByProtocol(order, item, model.DedicatedFeatureVless, cfg, tag)
	if !strings.Contains(vless, "#"+encodedTag) {
		t.Fatalf("vless link should contain encoded tag, got: %s", vless)
	}

	ss := buildOrderItemLinkByProtocol(order, item, model.DedicatedFeatureShadowsocks, cfg, tag)
	if !strings.Contains(ss, "#"+encodedTag) {
		t.Fatalf("shadowsocks link should contain encoded tag, got: %s", ss)
	}

	vmess := buildOrderItemLinkByProtocol(order, item, model.DedicatedFeatureVmess, cfg, tag)
	if !strings.HasPrefix(vmess, "vmess://") {
		t.Fatalf("unexpected vmess link: %s", vmess)
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(vmess, "vmess://"))
	if err != nil {
		t.Fatalf("decode vmess payload failed: %v", err)
	}
	var vmessJSON map[string]string
	if err := json.Unmarshal(payload, &vmessJSON); err != nil {
		t.Fatalf("unmarshal vmess payload failed: %v", err)
	}
	if strings.TrimSpace(vmessJSON["ps"]) != tag {
		t.Fatalf("vmess ps should be tag %q, got %q", tag, vmessJSON["ps"])
	}
}

func TestBuildSingleProtocolQRCodeImageLargerSize(t *testing.T) {
	img, err := buildSingleProtocolQRCodeImage("vless://11111111-2222-3333-4444-555555555555@line.example.com:443?security=tls#test", "")
	if err != nil {
		t.Fatalf("buildSingleProtocolQRCodeImage failed: %v", err)
	}
	if len(img) == 0 {
		t.Fatalf("expected non-empty qr image")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(img))
	if err != nil {
		t.Fatalf("decode qr image config failed: %v", err)
	}
	if format != "png" {
		t.Fatalf("expected png format, got %q", format)
	}
	if cfg.Width < 400 || cfg.Height < 400 {
		t.Fatalf("expected larger qr size, got %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.Width != cfg.Height {
		t.Fatalf("qr image should stay square, got %dx%d", cfg.Width, cfg.Height)
	}
}

func TestBuildOrdersXLSXDedicatedHeadersAndQRCodeCell(t *testing.T) {
	qr, err := buildSingleProtocolQRCodeImage("vless://11111111-2222-3333-4444-555555555555@line.example.com:443?security=tls#test", "美国-1.2.3.4")
	if err != nil {
		t.Fatalf("build qr failed: %v", err)
	}
	rows := []xlsxExportRow{
		{
			Mode:       model.OrderModeDedicated,
			OrderNo:    "OD260227000001",
			DomainLine: "line.example.com:443:user01:pass01",
			Link:       "vless://11111111-2222-3333-4444-555555555555@line.example.com:443?security=tls#test",
			RawSocks5:  "10.0.0.8:1080:user01:pass01",
			QRTag:      "美国-1.2.3.4",
			ExpiresAt:  time.Date(2026, 2, 27, 13, 14, 15, 0, time.UTC),
			QRCodeData: qr,
		},
	}
	body, err := buildOrdersXLSX(rows, model.DedicatedFeatureVless, true)
	if err != nil {
		t.Fatalf("buildOrdersXLSX failed: %v", err)
	}

	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("open xlsx failed: %v", err)
	}
	t.Cleanup(func() { _ = book.Close() })

	sheet := book.GetSheetName(0)
	headers, err := book.GetRows(sheet)
	if err != nil {
		t.Fatalf("get rows failed: %v", err)
	}
	if len(headers) == 0 {
		t.Fatalf("expected header row")
	}
	wantHeaders := []string{
		"Socks5 出口(IP:Port:User:Pass)",
		"入口Socks5(Domain:Port:User:Pass)",
		"专线链接",
		"二维码",
		"到期日",
		"订单号",
	}
	for i, expected := range wantHeaders {
		if i >= len(headers[0]) {
			t.Fatalf("missing header at position %d", i+1)
		}
		if strings.TrimSpace(headers[0][i]) != expected {
			t.Fatalf("unexpected header %d, got %q want %q", i+1, headers[0][i], expected)
		}
	}

	rowHeight, err := book.GetRowHeight(sheet, 2)
	if err != nil {
		t.Fatalf("get row height failed: %v", err)
	}
	if rowHeight < 180 {
		t.Fatalf("expected bigger qr row height, got %.2f", rowHeight)
	}

	inbound, err := book.GetCellValue(sheet, "B2")
	if err != nil {
		t.Fatalf("get inbound cell failed: %v", err)
	}
	if strings.TrimSpace(inbound) != "line.example.com:443:user01:pass01" {
		t.Fatalf("unexpected inbound cell: %q", inbound)
	}

	qrTag, err := book.GetCellValue(sheet, "D2")
	if err != nil {
		t.Fatalf("get qr tag cell failed: %v", err)
	}
	if strings.TrimSpace(qrTag) != "美国-1.2.3.4" {
		t.Fatalf("unexpected qr tag cell: %q", qrTag)
	}
	styleID, err := book.GetCellStyle(sheet, "D2")
	if err != nil {
		t.Fatalf("get qr tag style failed: %v", err)
	}
	style, err := book.GetStyle(styleID)
	if err != nil {
		t.Fatalf("get qr tag style detail failed: %v", err)
	}
	if style == nil || style.Alignment == nil {
		t.Fatalf("qr tag style alignment is missing")
	}
	if strings.ToLower(strings.TrimSpace(style.Alignment.Horizontal)) != "center" {
		t.Fatalf("qr tag should be horizontally centered, got: %q", style.Alignment.Horizontal)
	}
	if strings.ToLower(strings.TrimSpace(style.Alignment.Vertical)) != "top" {
		t.Fatalf("qr tag should be top aligned, got: %q", style.Alignment.Vertical)
	}

	orderNo, err := book.GetCellValue(sheet, "F2")
	if err != nil {
		t.Fatalf("get order_no cell failed: %v", err)
	}
	if strings.TrimSpace(orderNo) != "OD260227000001" {
		t.Fatalf("unexpected order_no cell: %q", orderNo)
	}

	linkValue, err := book.GetCellValue(sheet, "C2")
	if err != nil {
		t.Fatalf("get link cell failed: %v", err)
	}
	if strings.Contains(linkValue, "line.example.com:443:user01:pass01") {
		t.Fatalf("link cell should not merge socks5 outbound text, got: %q", linkValue)
	}

	pics, err := book.GetPictures(sheet, "D2")
	if err != nil {
		t.Fatalf("get pictures failed: %v", err)
	}
	if len(pics) == 0 || len(pics[0].File) == 0 {
		t.Fatalf("expected qr image attached in D2")
	}
	if pics[0].Format == nil || pics[0].Format.OffsetY < 20 {
		t.Fatalf("expected qr image moved down for top tag, offsetY=%v", func() int {
			if pics[0].Format == nil {
				return 0
			}
			return pics[0].Format.OffsetY
		}())
	}
}

func TestBuildOrdersXLSXDedicatedAlwaysSeparatesOutboundColumn(t *testing.T) {
	rows := []xlsxExportRow{
		{
			Mode:       model.OrderModeDedicated,
			OrderNo:    "OD260227000002",
			DomainLine: "line2.example.com:443:user02:pass02",
			Link:       "vmess://test-payload",
			RawSocks5:  "10.0.0.9:1080:user02:pass02",
			ExpiresAt:  time.Date(2026, 3, 1, 10, 11, 12, 0, time.UTC),
		},
	}
	body, err := buildOrdersXLSX(rows, model.DedicatedFeatureVmess, false)
	if err != nil {
		t.Fatalf("buildOrdersXLSX failed: %v", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("open xlsx failed: %v", err)
	}
	t.Cleanup(func() { _ = book.Close() })

	sheet := book.GetSheetName(0)
	head, err := book.GetCellValue(sheet, "A1")
	if err != nil {
		t.Fatalf("get A1 failed: %v", err)
	}
	if strings.TrimSpace(head) != "Socks5 出口(IP:Port:User:Pass)" {
		t.Fatalf("unexpected A1 header: %q", head)
	}
	raw, err := book.GetCellValue(sheet, "A2")
	if err != nil {
		t.Fatalf("get A2 failed: %v", err)
	}
	if strings.TrimSpace(raw) != "10.0.0.9:1080:user02:pass02" {
		t.Fatalf("unexpected A2 value: %q", raw)
	}
	inbound, err := book.GetCellValue(sheet, "B2")
	if err != nil {
		t.Fatalf("get B2 failed: %v", err)
	}
	if strings.TrimSpace(inbound) != "line2.example.com:443:user02:pass02" {
		t.Fatalf("unexpected B2 value: %q", inbound)
	}
	link, err := book.GetCellValue(sheet, "C2")
	if err != nil {
		t.Fatalf("get C2 failed: %v", err)
	}
	if strings.TrimSpace(link) != "vmess://test-payload" {
		t.Fatalf("unexpected C2 value: %q", link)
	}
}
