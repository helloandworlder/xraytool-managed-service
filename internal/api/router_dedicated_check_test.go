package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"xraytool/internal/service"
)

func TestCheckDedicatedSuccessReturnsConnectivityPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalProbe := dedicatedProtocolProbe
	t.Cleanup(func() {
		dedicatedProtocolProbe = originalProbe
	})

	dedicatedProtocolProbe = func(_ context.Context, req service.DedicatedProtocolProbeRequest) service.DedicatedProtocolProbeResult {
		if req.Protocol != "SOCKS5_MIXED" {
			t.Fatalf("unexpected protocol: %s", req.Protocol)
		}
		return service.DedicatedProtocolProbeResult{
			ConnectivityOK: true,
			ExitIP:         "8.8.8.8",
			CountryCode:    "us",
			Region:         "Virginia",
			Message:        "protocol probe succeeded",
		}
	}

	body, _ := json.Marshal(map[string]any{
		"routeType": "short_video",
		"protocol":  "socks5_mixed",
		"ip":        "127.0.0.1",
		"port":      1080,
		"username":  "u",
		"password":  "p",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/dedicated/check", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	api := &API{}
	api.checkDedicated(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if payload["connectivityOk"] != true {
		t.Fatalf("expected connectivityOk=true, got %+v", payload)
	}
	if payload["exitIp"] != "8.8.8.8" {
		t.Fatalf("expected exitIp, got %+v", payload)
	}
	if payload["protocol"] != "SOCKS5_MIXED" {
		t.Fatalf("expected normalized protocol, got %+v", payload)
	}
}

func TestCheckDedicatedFailureReturnsFalseConnectivity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalProbe := dedicatedProtocolProbe
	t.Cleanup(func() {
		dedicatedProtocolProbe = originalProbe
	})

	dedicatedProtocolProbe = func(_ context.Context, _ service.DedicatedProtocolProbeRequest) service.DedicatedProtocolProbeResult {
		return service.DedicatedProtocolProbeResult{
			ConnectivityOK: false,
			Message:        "protocol handshake failed",
			ErrorCode:      "PROTOCOL_HANDSHAKE_FAILED",
		}
	}

	body, _ := json.Marshal(map[string]any{
		"routeType": "short_video",
		"protocol":  "vmess",
		"ip":        "1.1.1.1",
		"port":      1080,
		"username":  "u",
		"password":  "p",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/dedicated/check", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	api := &API{}
	api.checkDedicated(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if payload["connectivityOk"] != false {
		t.Fatalf("expected connectivityOk=false, got %+v", payload)
	}
	if payload["errorCode"] != "PROTOCOL_HANDSHAKE_FAILED" {
		t.Fatalf("expected errorCode, got %+v", payload)
	}
}
