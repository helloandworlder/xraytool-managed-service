package service

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
)

func TestCreateDedicatedOrderDefersProbeUntilAfterCommit(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, &XrayManager{}, zap.NewNop())

	customer := model.Customer{Name: "async-customer", Code: "async", Status: model.OrderStatusActive}
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

	originalProbe := dedicatedEgressGeoProbe
	originalDispatch := dedicatedEgressAsyncDispatch
	defer func() {
		dedicatedEgressGeoProbe = originalProbe
		dedicatedEgressAsyncDispatch = originalDispatch
	}()

	var scheduled func()
	dedicatedEgressGeoProbe = func(address string, port int, username, password string) (string, string, string, error) {
		return "", "", "", errors.New("probe timeout")
	}
	dedicatedEgressAsyncDispatch = func(fn func()) {
		scheduled = fn
	}

	order, err := svc.CreateOrder(context.Background(), CreateOrderInput{
		CustomerID:         customer.ID,
		Name:               "async-dedicated",
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
		t.Fatalf("create dedicated order failed: %v", err)
	}
	if scheduled == nil {
		t.Fatalf("expected async probe to be scheduled")
	}

	child := model.Order{}
	if err := db.Where("parent_order_id = ?", order.ID).First(&child).Error; err != nil {
		t.Fatalf("load child order failed: %v", err)
	}
	egress := model.DedicatedEgress{}
	if err := db.Where("order_id = ?", child.ID).First(&egress).Error; err != nil {
		t.Fatalf("load dedicated egress failed: %v", err)
	}
	if egress.ProbeStatus != "pending" {
		t.Fatalf("probe_status = %q, want pending", egress.ProbeStatus)
	}
	if egress.LastProbedAt != nil {
		t.Fatalf("last_probed_at = %v, want nil before async probe runs", egress.LastProbedAt)
	}
	if egress.ExitIP != "" || egress.CountryCode != "" || egress.Region != "" {
		t.Fatalf("unexpected probe fields before async run: %+v", egress)
	}

	scheduled()

	if err := db.First(&egress, egress.ID).Error; err != nil {
		t.Fatalf("reload dedicated egress failed: %v", err)
	}
	if egress.ProbeStatus != "failed" {
		t.Fatalf("probe_status after async run = %q, want failed", egress.ProbeStatus)
	}
	if !strings.Contains(egress.ProbeError, "timeout") {
		t.Fatalf("probe_error = %q, want timeout", egress.ProbeError)
	}
	if egress.LastProbedAt == nil {
		t.Fatalf("last_probed_at should be set after async run")
	}
}

func TestProbeDedicatedEgressStreamRunsConcurrently(t *testing.T) {
	originalProbe := dedicatedEgressGeoProbe
	defer func() {
		dedicatedEgressGeoProbe = originalProbe
	}()

	var inFlight int32
	var maxInFlight int32
	dedicatedEgressGeoProbe = func(address string, port int, username, password string) (string, string, string, error) {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			seen := atomic.LoadInt32(&maxInFlight)
			if current <= seen || atomic.CompareAndSwapInt32(&maxInFlight, seen, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return "203.0.113.1", "us", "Virginia", nil
	}

	svc := &OrderService{}
	events := make([]DedicatedEgressProbeEvent, 0, 6)
	lines := strings.Join([]string{
		"1.1.1.1:1080:user01:pass01",
		"1.1.1.2:1080:user02:pass02",
		"1.1.1.3:1080:user03:pass03",
		"1.1.1.4:1080:user04:pass04",
	}, "\n")

	if err := svc.ProbeDedicatedEgressStream(lines, func(event DedicatedEgressProbeEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("probe stream failed: %v", err)
	}

	if atomic.LoadInt32(&maxInFlight) < 2 {
		t.Fatalf("expected concurrent probing, max in-flight = %d", atomic.LoadInt32(&maxInFlight))
	}
	resultCount := 0
	for _, event := range events {
		if event.Type == "result" {
			resultCount++
		}
	}
	if resultCount != 4 {
		t.Fatalf("result event count = %d, want 4", resultCount)
	}
}
