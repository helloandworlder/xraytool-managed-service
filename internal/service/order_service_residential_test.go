package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"xraytool/internal/config"
	"xraytool/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const residentialTestPort = 38457

func TestCreateResidentialOrderRejectsPrivateFallbackIPs(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, NewXrayManager(config.Config{}, db, zap.NewNop()), zap.NewNop())
	seedManagedAccountForPort(t, db, residentialTestPort)

	customer := model.Customer{Name: "home-private", Code: "hp", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	for _, host := range []model.HostIP{
		{IP: "192.168.207.2", IsPublic: false, IsLocal: true, Enabled: true},
		{IP: "127.0.0.1", IsPublic: false, IsLocal: true, Enabled: true},
	} {
		if err := db.Create(&host).Error; err != nil {
			t.Fatalf("create host ip %s failed: %v", host.IP, err)
		}
	}

	_, err := svc.CreateOrder(context.Background(), CreateOrderInput{
		CustomerID:  customer.ID,
		Name:        "home-order",
		Mode:        model.OrderModeAuto,
		Quantity:    1,
		DurationDay: 30,
		Port:        residentialTestPort,
	})
	if err == nil {
		t.Fatalf("expected create order to fail without public host ips")
	}
	if !strings.Contains(err.Error(), "no enabled public host ips") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateResidentialManualOrderRejectsPrivateIPs(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, NewXrayManager(config.Config{}, db, zap.NewNop()), zap.NewNop())
	seedManagedAccountForPort(t, db, residentialTestPort)

	customer := model.Customer{Name: "home-manual", Code: "hm", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	host := model.HostIP{IP: "192.168.207.9", IsPublic: false, IsLocal: true, Enabled: true}
	if err := db.Create(&host).Error; err != nil {
		t.Fatalf("create host ip failed: %v", err)
	}

	_, err := svc.CreateOrder(context.Background(), CreateOrderInput{
		CustomerID:  customer.ID,
		Name:        "home-manual-order",
		Mode:        model.OrderModeManual,
		Quantity:    1,
		DurationDay: 30,
		Port:        residentialTestPort,
		ManualIPIDs: []uint{host.ID},
	})
	if err == nil {
		t.Fatalf("expected manual create order to fail with private host ip")
	}
	if !strings.Contains(err.Error(), "not usable public local addresses") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateResidentialOrderUsesPublicHostIP(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, NewXrayManager(config.Config{}, db, zap.NewNop()), zap.NewNop())
	seedManagedAccountForPort(t, db, residentialTestPort)

	customer := model.Customer{Name: "home-public", Code: "hb", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	host := model.HostIP{IP: "203.0.113.10", IsPublic: true, IsLocal: true, Enabled: true}
	if err := db.Create(&host).Error; err != nil {
		t.Fatalf("create host ip failed: %v", err)
	}

	order, err := svc.CreateOrder(context.Background(), CreateOrderInput{
		CustomerID:  customer.ID,
		Name:        "home-public-order",
		Mode:        model.OrderModeAuto,
		Quantity:    1,
		DurationDay: 30,
		Port:        residentialTestPort,
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	if len(order.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(order.Items))
	}
	if order.Items[0].IP != host.IP {
		t.Fatalf("expected item ip %s, got %s", host.IP, order.Items[0].IP)
	}
}

func seedManagedAccountForPort(t *testing.T, db *gorm.DB, port int) {
	t.Helper()
	now := time.Now()
	customer := model.Customer{Name: "seed-port-customer", Code: "seed-port", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create seed customer failed: %v", err)
	}
	order := model.Order{
		CustomerID: customer.ID,
		Name:       "seed-port-order",
		Mode:       model.OrderModeAuto,
		Status:     model.OrderStatusActive,
		Quantity:   1,
		Port:       port,
		StartsAt:   now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create seed order failed: %v", err)
	}
	item := model.OrderItem{
		OrderID:   order.ID,
		IP:        "203.0.113.200",
		Port:      port,
		Username:  "seed-user",
		Password:  "seed-pass",
		Managed:   true,
		Status:    model.OrderItemStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create seed order item failed: %v", err)
	}
}
