package service

import (
	"testing"

	"xraytool/internal/model"
)

func TestParseDedicatedCredentialLinesForProtocolExtractsUUIDFromCompatLine(t *testing.T) {
	rows, err := parseDedicatedCredentialLinesForProtocol(
		"user01:pass01:11111111-2222-3333-4444-555555555555",
		model.DedicatedFeatureVmess,
	)
	if err != nil {
		t.Fatalf("parseDedicatedCredentialLinesForProtocol failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].UUID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("unexpected uuid: %q", rows[0].UUID)
	}
	if rows[0].Username != "user01" || rows[0].Password != "pass01" {
		t.Fatalf("expected username/password to remain parsed, got %#v", rows[0])
	}
}

func TestParseDedicatedCredentialLinesForProtocolAllowsPlainUUID(t *testing.T) {
	rows, err := parseDedicatedCredentialLinesForProtocol(
		"11111111-2222-3333-4444-555555555555",
		model.DedicatedFeatureVless,
	)
	if err != nil {
		t.Fatalf("parseDedicatedCredentialLinesForProtocol failed: %v", err)
	}
	if len(rows) != 1 || rows[0].UUID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}
