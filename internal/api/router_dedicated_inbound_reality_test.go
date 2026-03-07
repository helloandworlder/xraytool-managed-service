package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"xraytool/internal/service"
)

func TestGenerateRealityKeyPairReturnsMatchingPair(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/orders/dedicated-inbounds/reality-keypair", nil)

	api := &API{}
	api.generateRealityKeyPair(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		OK         bool   `json:"ok"`
		PrivateKey string `json:"private_key"`
		PublicKey  string `json:"public_key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !payload.OK || payload.PrivateKey == "" || payload.PublicKey == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	validatedPublic, deriveErr := service.ValidateDedicatedInboundInput(service.DedicatedInboundInput{
		Protocol:          "vless",
		ListenPort:        443,
		VlessSecurity:     "reality",
		VlessSNI:          "www.tesla.com",
		RealityTarget:     "www.tesla.com:443",
		RealityPrivateKey: payload.PrivateKey,
	})
	if deriveErr != nil {
		t.Fatalf("ValidateDedicatedInboundInput failed: %v", deriveErr)
	}
	if validatedPublic.RealityPublicKey != payload.PublicKey {
		t.Fatalf("public key mismatch, got %q want %q", payload.PublicKey, validatedPublic.RealityPublicKey)
	}
}

func TestValidateDedicatedInboundReturnsGeneratedRealityKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(map[string]any{
		"protocol":       "vless",
		"listen_port":    443,
		"vless_security": "reality",
		"vless_sni":      "www.tesla.com",
		"reality_target": "www.tesla.com:443",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/orders/dedicated-inbounds/validate", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	api := &API{}
	api.validateDedicatedInbound(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		OK      bool `json:"ok"`
		Inbound struct {
			RealityPrivateKey string `json:"reality_private_key"`
			RealityPublicKey  string `json:"reality_public_key"`
			VlessFingerprint  string `json:"vless_fingerprint"`
		} `json:"inbound"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !payload.OK || payload.Inbound.RealityPrivateKey == "" || payload.Inbound.RealityPublicKey == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Inbound.VlessFingerprint != "chrome" {
		t.Fatalf("unexpected fingerprint: %+v", payload)
	}
}

func TestValidateDedicatedInboundRejectsInvalidRealityShortID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(map[string]any{
		"protocol":            "vless",
		"listen_port":         443,
		"vless_security":      "reality",
		"vless_sni":           "www.tesla.com",
		"reality_target":      "www.tesla.com:443",
		"reality_private_key": "k0d_DrM8TU4v7a0Vh3lTcrQ7xjJ7Qm4-EtaVB0Wk4gs",
		"reality_short_ids":   "abc",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/orders/dedicated-inbounds/validate", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	api := &API{}
	api.validateDedicatedInbound(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}
