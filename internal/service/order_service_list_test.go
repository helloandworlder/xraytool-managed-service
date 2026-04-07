package service

import (
	"testing"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
)

func TestListOrdersSupportsPaginationAndFilters(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, &XrayManager{}, zap.NewNop())

	now := time.Now()
	customerA := model.Customer{Name: "Alpha", Code: "ALP", Status: model.OrderStatusActive}
	customerB := model.Customer{Name: "Beta", Code: "BET", Status: model.OrderStatusActive}
	if err := db.Create(&customerA).Error; err != nil {
		t.Fatalf("create customer A failed: %v", err)
	}
	if err := db.Create(&customerB).Error; err != nil {
		t.Fatalf("create customer B failed: %v", err)
	}

	standalone := model.Order{
		OrderNo:    "OD-A-001",
		CustomerID: customerA.ID,
		Name:       "Alpha Home",
		Mode:       model.OrderModeAuto,
		Status:     model.OrderStatusActive,
		Quantity:   1,
		Port:       23457,
		StartsAt:   now,
		ExpiresAt:  now.Add(30 * 24 * time.Hour),
	}
	if err := db.Create(&standalone).Error; err != nil {
		t.Fatalf("create standalone order failed: %v", err)
	}
	if err := db.Create(&model.OrderItem{
		OrderID:   standalone.ID,
		IP:        "10.0.0.1",
		Port:      23457,
		Username:  "alpha-user",
		Password:  "alpha-pass",
		Managed:   true,
		Status:    model.OrderItemStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create standalone item failed: %v", err)
	}

	groupHead := model.Order{
		OrderNo:     "OD-B-HEAD",
		CustomerID:  customerB.ID,
		GroupID:     0,
		IsGroupHead: true,
		Name:        "Beta Dedicated",
		Mode:        model.OrderModeDedicated,
		Status:      model.OrderStatusActive,
		Quantity:    2,
		Port:        1080,
		StartsAt:    now,
		ExpiresAt:   now.Add(60 * 24 * time.Hour),
	}
	if err := db.Create(&groupHead).Error; err != nil {
		t.Fatalf("create group head failed: %v", err)
	}
	if err := db.Model(&model.Order{}).Where("id = ?", groupHead.ID).Update("group_id", groupHead.ID).Error; err != nil {
		t.Fatalf("update group head group_id failed: %v", err)
	}

	for i, ip := range []string{"203.0.113.10", "203.0.113.11"} {
		parentID := groupHead.ID
		child := model.Order{
			OrderNo:       "OD-B-CHILD-" + string(rune('1'+i)),
			CustomerID:    customerB.ID,
			GroupID:       groupHead.ID,
			ParentOrderID: &parentID,
			SequenceNo:    i + 1,
			Name:          "Beta Child",
			Mode:          model.OrderModeDedicated,
			Status:        model.OrderStatusActive,
			Quantity:      1,
			Port:          1080,
			StartsAt:      now,
			ExpiresAt:     now.Add(60 * 24 * time.Hour),
		}
		if err := db.Create(&child).Error; err != nil {
			t.Fatalf("create child order failed: %v", err)
		}
		if err := db.Create(&model.OrderItem{
			OrderID:         child.ID,
			IP:              "127.0.0.1",
			Port:            1080,
			Username:        "child-user",
			Password:        "child-pass",
			ForwardAddress:  ip,
			ForwardPort:     2080 + i,
			ForwardUsername: "egress-user",
			ForwardPassword: "egress-pass",
			Managed:         true,
			Status:          model.OrderItemStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		}).Error; err != nil {
			t.Fatalf("create child item failed: %v", err)
		}
	}

	disabled := model.Order{
		OrderNo:    "OD-C-001",
		CustomerID: customerA.ID,
		Name:       "Disabled Home",
		Mode:       model.OrderModeManual,
		Status:     model.OrderStatusDisabled,
		Quantity:   1,
		Port:       34567,
		StartsAt:   now,
		ExpiresAt:  now.Add(15 * 24 * time.Hour),
	}
	if err := db.Create(&disabled).Error; err != nil {
		t.Fatalf("create disabled order failed: %v", err)
	}
	if err := db.Create(&model.OrderItem{
		OrderID:   disabled.ID,
		IP:        "10.0.0.2",
		Port:      34567,
		Username:  "disabled-user",
		Password:  "disabled-pass",
		Managed:   true,
		Status:    model.OrderItemStatusDisabled,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create disabled item failed: %v", err)
	}

	pageOne, err := svc.ListOrders(ListOrdersInput{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("list orders page one failed: %v", err)
	}
	if pageOne.Total != 3 {
		t.Fatalf("expected 3 root orders, got %d", pageOne.Total)
	}
	if len(pageOne.Rows) != 1 || pageOne.Rows[0].ID != disabled.ID {
		t.Fatalf("expected page one to contain latest standalone root order")
	}
	if pageOne.Stats.Total != 5 || pageOne.Stats.Active != 4 || pageOne.Stats.Disabled != 1 {
		t.Fatalf("unexpected stats: %+v", pageOne.Stats)
	}

	pageTwo, err := svc.ListOrders(ListOrdersInput{Page: 2, PageSize: 1})
	if err != nil {
		t.Fatalf("list orders page two failed: %v", err)
	}
	if len(pageTwo.Rows) != 3 {
		t.Fatalf("expected group head plus 2 children on page two, got %d rows", len(pageTwo.Rows))
	}
	if pageTwo.Rows[0].ID != groupHead.ID {
		t.Fatalf("expected group head to be first row on second page")
	}
	if pageTwo.Rows[1].ParentOrderID == nil || *pageTwo.Rows[1].ParentOrderID != groupHead.ID {
		t.Fatalf("expected second row to be child order")
	}

	customerFiltered, err := svc.ListOrders(ListOrdersInput{Page: 1, PageSize: 12, CustomerID: customerB.ID})
	if err != nil {
		t.Fatalf("list orders by customer failed: %v", err)
	}
	if customerFiltered.Total != 1 || len(customerFiltered.Rows) != 3 {
		t.Fatalf("expected one root group for customer B, got total=%d rows=%d", customerFiltered.Total, len(customerFiltered.Rows))
	}

	keywordFiltered, err := svc.ListOrders(ListOrdersInput{Page: 1, PageSize: 12, Keyword: "203.0.113.11"})
	if err != nil {
		t.Fatalf("list orders by child item keyword failed: %v", err)
	}
	if keywordFiltered.Total != 1 || len(keywordFiltered.Rows) != 3 {
		t.Fatalf("expected keyword to match grouped order through child item, got total=%d rows=%d", keywordFiltered.Total, len(keywordFiltered.Rows))
	}
	if keywordFiltered.Rows[0].ID != groupHead.ID {
		t.Fatalf("expected keyword match to return group head row first")
	}
}
