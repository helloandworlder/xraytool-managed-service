package service

import "testing"

func TestParseDedicatedEgressLinesTrimPrefix(t *testing.T) {
	rows, err := parseDedicatedEgressLines("~74.2.103.133:47279:userA:passA")
	if err != nil {
		t.Fatalf("parseDedicatedEgressLines returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Address != "74.2.103.133" {
		t.Fatalf("expected trimmed address, got %q", rows[0].Address)
	}
}

func TestParseDedicatedEgressGeoLinesPipeFormat(t *testing.T) {
	lines := "74.2.103.133:47279:userA:passA|US|Virginia\n74.2.103.17:47279:userB:passB|MX|Jalisco"
	rows, err := parseDedicatedEgressGeoLines(lines, "", "")
	if err != nil {
		t.Fatalf("parseDedicatedEgressGeoLines returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].CountryCode != "us" || rows[0].Region != "Virginia" {
		t.Fatalf("unexpected first row geo: %+v", rows[0])
	}
	if rows[1].CountryCode != "mx" || rows[1].Region != "Jalisco" {
		t.Fatalf("unexpected second row geo: %+v", rows[1])
	}
}

func TestParseDedicatedEgressGeoLinesFallbackDefaults(t *testing.T) {
	rows, err := parseDedicatedEgressGeoLines("74.2.103.175:47279:userC:passC", "US", "Texas")
	if err != nil {
		t.Fatalf("parseDedicatedEgressGeoLines returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].CountryCode != "us" || rows[0].Region != "Texas" {
		t.Fatalf("unexpected fallback geo: %+v", rows[0])
	}
}

func TestParseDedicatedEgressLinesPastedBatch(t *testing.T) {
	lines := "~74.2.103.133:47279:3sRFIHhR:Q4HiCaJNCRz9\n74.2.103.17:47279:4SzCtB02:XaAicUPRTDE4\n74.2.103.175:47279:m5oAJRTl:AeCFG3xWKpUJ\n74.2.103.179:47279:qrZO2zJM:P7DUlNAXHza3\n74.2.103.185:47279:HK5L2NqR:R1MV5MOi817K\n74.2.103.21:47279:U5NgMchN:cZRt50QCs6qa\n74.2.103.47:47279:QDcS3R4H:TyhShwK2dQ3l\n74.2.103.49:47279:FJbACeJp:1dBVjFuTSYpj\n74.2.103.63:47279:mbElQhy6:B7tOSSO3RqKL"
	rows, err := parseDedicatedEgressLines(lines)
	if err != nil {
		t.Fatalf("parseDedicatedEgressLines returned error: %v", err)
	}
	if len(rows) != 9 {
		t.Fatalf("expected 9 rows, got %d", len(rows))
	}
	if rows[0].Address != "74.2.103.133" {
		t.Fatalf("expected first address trimmed, got %q", rows[0].Address)
	}
	if rows[8].Address != "74.2.103.63" {
		t.Fatalf("unexpected last row address: %q", rows[8].Address)
	}
}
