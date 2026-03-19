package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
)

func TestCreateDedicatedOrderRejectsDuplicateEgressForSameCustomer(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, &XrayManager{}, zap.NewNop())

	now := time.Now()
	customer := model.Customer{Name: "dup-customer", Code: "dup", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	entry := model.DedicatedEntry{Name: "entry", Domain: "entry.example.com", MixedPort: 1080, VmessPort: 443, VlessPort: 443, ShadowsocksPort: 443, Features: "mixed,vmess,vless,shadowsocks", Enabled: true}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create entry failed: %v", err)
	}
	inbound := model.DedicatedInbound{Name: "vless-in", Protocol: model.DedicatedFeatureVless, ListenPort: 443, Enabled: true, VlessSecurity: "tls"}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound failed: %v", err)
	}
	ingress := model.DedicatedIngress{Name: "ingress", DedicatedInboundID: inbound.ID, Domain: "line.example.com", IngressPort: 443, Enabled: true}
	if err := db.Create(&ingress).Error; err != nil {
		t.Fatalf("create ingress failed: %v", err)
	}

	_, err := svc.CreateOrder(context.Background(), CreateOrderInput{
		CustomerID:         customer.ID,
		Name:               "first",
		Mode:               model.OrderModeDedicated,
		DurationDay:        30,
		DedicatedProtocol:  model.DedicatedFeatureVless,
		DedicatedEntryID:   entry.ID,
		DedicatedInboundID: inbound.ID,
		DedicatedIngressID: ingress.ID,
		DedicatedEgressLines: strings.Join([]string{
			"1.1.1.1:1080:user01:pass01",
		}, "\n"),
	})
	if err != nil {
		t.Fatalf("first dedicated create failed: %v", err)
	}

	_, err = svc.CreateOrder(context.Background(), CreateOrderInput{
		CustomerID:         customer.ID,
		Name:               "second",
		Mode:               model.OrderModeDedicated,
		ExpiresAt:          now.Add(30 * 24 * time.Hour),
		DedicatedProtocol:  model.DedicatedFeatureVless,
		DedicatedEntryID:   entry.ID,
		DedicatedInboundID: inbound.ID,
		DedicatedIngressID: ingress.ID,
		DedicatedEgressLines: strings.Join([]string{
			"1.1.1.1:1080:user01:pass01",
		}, "\n"),
	})
	if err == nil {
		t.Fatalf("expected duplicate dedicated egress to be rejected")
	}
	if !strings.Contains(err.Error(), "already used by current customer") {
		t.Fatalf("unexpected error: %v", err)
	}
}
