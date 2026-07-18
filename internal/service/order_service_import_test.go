package service

import (
	"context"
	"testing"
	"time"

	"xraytool/internal/config"
	"xraytool/internal/model"

	"go.uber.org/zap"
)

func TestImportOrderUsesExternalReferenceAsIdempotencyKey(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, NewXrayManager(config.Config{}, db, zap.NewNop()), zap.NewNop())

	customer := model.Customer{Name: "import-idempotency", Code: "import-idempotency", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	if err := db.Create(&model.HostIP{IP: "203.0.113.88", IsPublic: true, IsLocal: true, Enabled: true}).Error; err != nil {
		t.Fatalf("create host ip failed: %v", err)
	}

	rows := []ImportPreviewRow{{IP: "203.0.113.88", Port: residentialTestPort, Username: "import-user", Password: "import-pass"}}
	first, err := svc.ImportOrder(context.Background(), customer.ID, "import-first", time.Now().Add(24*time.Hour), "gosea-reference-1", rows)
	if err != nil {
		t.Fatalf("first import failed: %v", err)
	}
	second, err := svc.ImportOrder(context.Background(), customer.ID, "import-retry", time.Now().Add(24*time.Hour), "gosea-reference-1", rows)
	if err != nil {
		t.Fatalf("retry import failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected retry to return order %d, got %d", first.ID, second.ID)
	}
	var count int64
	if err := db.Model(&model.Order{}).Count(&count).Error; err != nil {
		t.Fatalf("count orders failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one imported order, got %d", count)
	}
}
