package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"xraytool/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUpdateGroupEgressGeoByMapping(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, nil, nil)
	headID, children := seedDedicatedGroupForGeoTest(t, db)

	err := svc.UpdateGroupEgressGeoByMapping(context.Background(), headID, "10.0.0.1:1080:u1:p1|US|Virginia\n10.0.0.2:1080:u2:p2|MX|Jalisco", "", "")
	if err != nil {
		t.Fatalf("UpdateGroupEgressGeoByMapping returned error: %v", err)
	}

	var e1 model.DedicatedEgress
	if err := db.Where("order_item_id = ?", children[0].itemID).First(&e1).Error; err != nil {
		t.Fatalf("load egress 1 failed: %v", err)
	}
	if e1.CountryCode != "us" || e1.Region != "Virginia" {
		t.Fatalf("unexpected egress 1 geo: %+v", e1)
	}

	var e2 model.DedicatedEgress
	if err := db.Where("order_item_id = ?", children[1].itemID).First(&e2).Error; err != nil {
		t.Fatalf("load egress 2 failed: %v", err)
	}
	if e2.CountryCode != "mx" || e2.Region != "Jalisco" {
		t.Fatalf("unexpected egress 2 geo: %+v", e2)
	}
}

func TestUpdateGroupEgressGeoSelected(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, nil, nil)
	headID, children := seedDedicatedGroupForGeoTest(t, db)

	err := svc.UpdateGroupEgressGeo(context.Background(), headID, []uint{children[0].orderID}, "CA", "Ontario")
	if err != nil {
		t.Fatalf("UpdateGroupEgressGeo returned error: %v", err)
	}

	var e1 model.DedicatedEgress
	if err := db.Where("order_item_id = ?", children[0].itemID).First(&e1).Error; err != nil {
		t.Fatalf("load egress 1 failed: %v", err)
	}
	if e1.CountryCode != "ca" || e1.Region != "Ontario" {
		t.Fatalf("unexpected egress 1 geo: %+v", e1)
	}

	var e2 model.DedicatedEgress
	err = db.Where("order_item_id = ?", children[1].itemID).First(&e2).Error
	if err == nil {
		t.Fatalf("expected second egress to remain untouched and absent, got: %+v", e2)
	}
}

type geoSeedChild struct {
	orderID uint
	itemID  uint
}

func seedDedicatedGroupForGeoTest(t *testing.T, db *gorm.DB) (uint, []geoSeedChild) {
	t.Helper()
	now := time.Now()
	customer := model.Customer{Name: "c1", Code: "c1", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	head := model.Order{
		CustomerID:        customer.ID,
		Name:              "head",
		Mode:              model.OrderModeDedicated,
		Status:            model.OrderStatusActive,
		Quantity:          2,
		Port:              1080,
		StartsAt:          now,
		ExpiresAt:         now.Add(24 * time.Hour),
		IsGroupHead:       true,
		GroupID:           1,
		DedicatedProtocol: model.DedicatedFeatureMixed,
	}
	if err := db.Create(&head).Error; err != nil {
		t.Fatalf("create head failed: %v", err)
	}
	if err := db.Model(&model.Order{}).Where("id = ?", head.ID).Update("group_id", head.ID).Error; err != nil {
		t.Fatalf("update head group id failed: %v", err)
	}

	children := make([]geoSeedChild, 0, 2)
	inputs := []struct {
		address  string
		username string
		password string
	}{
		{address: "10.0.0.1", username: "u1", password: "p1"},
		{address: "10.0.0.2", username: "u2", password: "p2"},
	}
	for idx, in := range inputs {
		parentID := head.ID
		child := model.Order{
			CustomerID:        customer.ID,
			GroupID:           head.ID,
			ParentOrderID:     &parentID,
			SequenceNo:        idx + 1,
			Name:              "child",
			Mode:              model.OrderModeDedicated,
			Status:            model.OrderStatusActive,
			Quantity:          1,
			Port:              1080,
			StartsAt:          now,
			ExpiresAt:         now.Add(24 * time.Hour),
			DedicatedProtocol: model.DedicatedFeatureMixed,
		}
		if err := db.Create(&child).Error; err != nil {
			t.Fatalf("create child failed: %v", err)
		}
		item := model.OrderItem{
			OrderID:         child.ID,
			IP:              "127.0.0.1",
			Port:            1080,
			Username:        "x",
			Password:        "y",
			ForwardAddress:  in.address,
			ForwardPort:     1080,
			ForwardUsername: in.username,
			ForwardPassword: in.password,
			Managed:         true,
			Status:          model.OrderItemStatusActive,
			OutboundType:    model.OutboundTypeSocks5,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create item failed: %v", err)
		}
		children = append(children, geoSeedChild{orderID: child.ID, itemID: item.ID})
	}
	return head.ID, children
}

func setupOrderServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Customer{}, &model.DedicatedEntry{}, &model.DedicatedInbound{}, &model.DedicatedIngress{}, &model.Order{}, &model.OrderItem{}, &model.DedicatedEgress{}); err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}
	return db
}
