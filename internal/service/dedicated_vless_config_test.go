package service

import (
	"strings"
	"testing"

	"xraytool/internal/model"
)

func TestGenerateRealityKeyPairProducesMatchingPublicKey(t *testing.T) {
	privateKey, publicKey, err := GenerateRealityKeyPair()
	if err != nil {
		t.Fatalf("GenerateRealityKeyPair failed: %v", err)
	}
	if strings.TrimSpace(privateKey) == "" || strings.TrimSpace(publicKey) == "" {
		t.Fatalf("expected non-empty key pair")
	}
	derived, err := deriveRealityPublicKey(privateKey)
	if err != nil {
		t.Fatalf("deriveRealityPublicKey failed: %v", err)
	}
	if derived != publicKey {
		t.Fatalf("public key mismatch, got %q want %q", publicKey, derived)
	}
}

func TestValidateDedicatedInboundInputAutoGeneratesRealityPrivateKey(t *testing.T) {
	row, err := ValidateDedicatedInboundInput(DedicatedInboundInput{
		Protocol:      model.DedicatedFeatureVless,
		ListenPort:    443,
		VlessSecurity: dedicatedVlessSecurityReality,
		VlessSNI:      "www.tesla.com",
		RealityTarget: "www.tesla.com:443",
	})
	if err != nil {
		t.Fatalf("ValidateDedicatedInboundInput failed: %v", err)
	}
	if strings.TrimSpace(row.RealityPrivateKey) == "" {
		t.Fatalf("expected generated private key")
	}
	if strings.TrimSpace(row.RealityPublicKey) == "" {
		t.Fatalf("expected generated public key")
	}
	if row.VlessFingerprint != defaultRealityFingerprint {
		t.Fatalf("expected default fingerprint %q, got %q", defaultRealityFingerprint, row.VlessFingerprint)
	}
}

func TestValidateDedicatedInboundInputRejectsInvalidRealityShortID(t *testing.T) {
	_, err := ValidateDedicatedInboundInput(DedicatedInboundInput{
		Protocol:          model.DedicatedFeatureVless,
		ListenPort:        443,
		VlessSecurity:     dedicatedVlessSecurityReality,
		VlessSNI:          "www.tesla.com",
		RealityTarget:     "www.tesla.com:443",
		RealityShortIDs:   "abc",
		RealityPrivateKey: "k0d_DrM8TU4v7a0Vh3lTcrQ7xjJ7Qm4-EtaVB0Wk4gs",
	})
	if err == nil {
		t.Fatalf("expected invalid short id error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "short id") {
		t.Fatalf("unexpected error: %v", err)
	}
}
