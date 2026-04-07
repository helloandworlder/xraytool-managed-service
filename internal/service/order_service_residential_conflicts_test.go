package service

import (
	"context"
	"testing"
	"time"

	"xraytool/internal/config"
	"xraytool/internal/model"

	"go.uber.org/zap"
)

func TestListAndRepairResidentialUsernameConflicts(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, NewXrayManager(config.Config{}, db, zap.NewNop()), zap.NewNop())
	seedManagedAccountForPort(t, db, 39001)

	customerA := model.Customer{Name: "Conflict A", Code: "CA", Status: model.OrderStatusActive}
	customerB := model.Customer{Name: "Conflict B", Code: "CB", Status: model.OrderStatusActive}
	if err := db.Create(&customerA).Error; err != nil {
		t.Fatalf("create customer A failed: %v", err)
	}
	if err := db.Create(&customerB).Error; err != nil {
		t.Fatalf("create customer B failed: %v", err)
	}

	hostA := model.HostIP{IP: "203.0.113.71", IsPublic: true, IsLocal: true, Enabled: true}
	hostB := model.HostIP{IP: "203.0.113.72", IsPublic: true, IsLocal: true, Enabled: true}
	if err := db.Create(&hostA).Error; err != nil {
		t.Fatalf("create host A failed: %v", err)
	}
	if err := db.Create(&hostB).Error; err != nil {
		t.Fatalf("create host B failed: %v", err)
	}

	now := time.Now()
	orderA := model.Order{OrderNo: "OD-CONFLICT-A", CustomerID: customerA.ID, Name: "Order A", Mode: model.OrderModeManual, Status: model.OrderStatusActive, Quantity: 1, Port: 39001, StartsAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	orderB := model.Order{OrderNo: "OD-CONFLICT-B", CustomerID: customerB.ID, Name: "Order B", Mode: model.OrderModeManual, Status: model.OrderStatusActive, Quantity: 1, Port: 39002, StartsAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err := db.Create(&orderA).Error; err != nil {
		t.Fatalf("create order A failed: %v", err)
	}
	if err := db.Create(&orderB).Error; err != nil {
		t.Fatalf("create order B failed: %v", err)
	}

	if err := db.Create(&model.OrderItem{
		OrderID:   orderA.ID,
		HostIPID:  &hostA.ID,
		IP:        hostA.IP,
		Port:      39001,
		Username:  "dup-home-user",
		Password:  "pass-a",
		Managed:   true,
		Status:    model.OrderItemStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create order item A failed: %v", err)
	}
	if err := db.Create(&model.OrderItem{
		OrderID:   orderB.ID,
		HostIPID:  &hostB.ID,
		IP:        hostB.IP,
		Port:      39002,
		Username:  "dup-home-user",
		Password:  "pass-b",
		Managed:   true,
		Status:    model.OrderItemStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create order item B failed: %v", err)
	}

	conflicts, err := svc.ListResidentialUsernameConflicts()
	if err != nil {
		t.Fatalf("list conflicts failed: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Username != "dup-home-user" || conflicts[0].OrderCount != 2 {
		t.Fatalf("unexpected conflict row: %+v", conflicts[0])
	}

	results := svc.RepairResidentialUsernameConflicts(context.Background(), conflicts[0].AffectedOrderIDs)
	if len(results) != 2 {
		t.Fatalf("expected 2 repair results, got %d", len(results))
	}
	for _, row := range results {
		if !row.Success {
			t.Fatalf("expected repair success, got %+v", row)
		}
	}

	conflictsAfter, err := svc.ListResidentialUsernameConflicts()
	if err != nil {
		t.Fatalf("list conflicts after repair failed: %v", err)
	}
	if len(conflictsAfter) != 0 {
		t.Fatalf("expected conflicts to be repaired, got %+v", conflictsAfter)
	}
}
