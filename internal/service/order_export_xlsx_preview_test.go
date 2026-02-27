package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"xraytool/internal/model"
)

func TestWritePreviewXLSXFile(t *testing.T) {
	outputPath := strings.TrimSpace(os.Getenv("XRAYTOOL_PREVIEW_XLSX_PATH"))
	if outputPath == "" {
		t.Skip("set XRAYTOOL_PREVIEW_XLSX_PATH to enable preview xlsx generation")
	}
	qr, err := buildSingleProtocolQRCodeImage("vless://11111111-2222-3333-4444-555555555555@line.example.com:443?security=tls#test", "")
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
	if err := os.WriteFile(outputPath, body, 0644); err != nil {
		t.Fatalf("write xlsx failed: %v", err)
	}
	t.Logf("preview xlsx written: %s", outputPath)
}
