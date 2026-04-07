package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"xraytool/internal/model"
)

func TestRuntimeOverviewPersistsWindowsAcrossServiceRestart(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	now := time.Date(2026, 4, 7, 18, 30, 0, 0, time.UTC)

	customer := model.Customer{Name: "daoge", Code: "daoge", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}

	homeOrder := model.Order{
		OrderNo:    "OD-HOME-001",
		CustomerID: customer.ID,
		Name:       "Home Socks5",
		Mode:       model.OrderModeAuto,
		Status:     model.OrderStatusActive,
		Quantity:   1,
		Port:       23457,
		StartsAt:   now.Add(-24 * time.Hour),
		ExpiresAt:  now.Add(24 * time.Hour),
	}
	if err := db.Create(&homeOrder).Error; err != nil {
		t.Fatalf("create home order failed: %v", err)
	}
	if err := db.Create(&model.OrderItem{
		OrderID:   homeOrder.ID,
		IP:        "10.0.0.1",
		Port:      23457,
		Username:  "home-user",
		Password:  "home-pass",
		Managed:   true,
		Status:    model.OrderItemStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create home item failed: %v", err)
	}

	groupHead := model.Order{
		OrderNo:     "OD-GRP-001",
		CustomerID:  customer.ID,
		IsGroupHead: true,
		Name:        "Dedicated Group",
		Mode:        model.OrderModeDedicated,
		Status:      model.OrderStatusActive,
		Quantity:    1,
		Port:        1080,
		StartsAt:    now.Add(-24 * time.Hour),
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	if err := db.Create(&groupHead).Error; err != nil {
		t.Fatalf("create group head failed: %v", err)
	}
	if err := db.Model(&model.Order{}).Where("id = ?", groupHead.ID).Update("group_id", groupHead.ID).Error; err != nil {
		t.Fatalf("update group_id failed: %v", err)
	}
	parentID := groupHead.ID
	childOrder := model.Order{
		OrderNo:       "OD-GRP-001-01",
		CustomerID:    customer.ID,
		GroupID:       groupHead.ID,
		ParentOrderID: &parentID,
		SequenceNo:    1,
		Name:          "Dedicated Child",
		Mode:          model.OrderModeDedicated,
		Status:        model.OrderStatusActive,
		Quantity:      1,
		Port:          1080,
		StartsAt:      now.Add(-24 * time.Hour),
		ExpiresAt:     now.Add(24 * time.Hour),
	}
	if err := db.Create(&childOrder).Error; err != nil {
		t.Fatalf("create child order failed: %v", err)
	}
	if err := db.Create(&model.OrderItem{
		OrderID:   childOrder.ID,
		IP:        "127.0.0.1",
		Port:      1080,
		Username:  "dedicated-user",
		Password:  "dedicated-pass",
		Managed:   true,
		Status:    model.OrderItemStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create dedicated item failed: %v", err)
	}

	serviceAtT1 := NewRuntimeStatsService(db, nil)
	t1 := now.Add(-30 * time.Minute)
	serviceAtT1.nowFn = func() time.Time { return t1 }
	serviceAtT1.trafficProvider = func(context.Context) (map[string]int64, error) {
		return map[string]int64{
			"user>>>home-user>>>traffic>>>uplink":        100,
			"user>>>home-user>>>traffic>>>downlink":      200,
			"user>>>dedicated-user>>>traffic>>>uplink":   400,
			"user>>>dedicated-user>>>traffic>>>downlink": 600,
		}, nil
	}
	serviceAtT1.onlineListProvider = func(context.Context) ([]string, error) {
		return []string{
			"user>>>home-user>>>online",
			"user>>>dedicated-user>>>online",
		}, nil
	}
	serviceAtT1.onlineCountsProvider = func(_ context.Context, statNames []string) (map[string]int64, error) {
		out := map[string]int64{}
		for _, statName := range statNames {
			if strings.Contains(statName, "home-user") {
				out[statName] = 1
				continue
			}
			out[statName] = 3
		}
		return out, nil
	}
	if _, err := serviceAtT1.Overview(context.Background(), 30); err != nil {
		t.Fatalf("overview at t1 failed: %v", err)
	}

	serviceAtT2 := NewRuntimeStatsService(db, nil)
	serviceAtT2.nowFn = func() time.Time { return now }
	serviceAtT2.trafficProvider = func(context.Context) (map[string]int64, error) {
		return map[string]int64{
			"user>>>home-user>>>traffic>>>uplink":        700,
			"user>>>home-user>>>traffic>>>downlink":      900,
			"user>>>dedicated-user>>>traffic>>>uplink":   2000,
			"user>>>dedicated-user>>>traffic>>>downlink": 3000,
		}, nil
	}
	serviceAtT2.onlineListProvider = func(context.Context) ([]string, error) {
		return []string{
			"user>>>home-user>>>online",
			"user>>>dedicated-user>>>online",
		}, nil
	}
	serviceAtT2.onlineCountsProvider = func(_ context.Context, statNames []string) (map[string]int64, error) {
		out := map[string]int64{}
		for _, statName := range statNames {
			if strings.Contains(statName, "home-user") {
				out[statName] = 2
				continue
			}
			out[statName] = 4
		}
		return out, nil
	}

	overview, err := serviceAtT2.Overview(context.Background(), 30)
	if err != nil {
		t.Fatalf("overview at t2 failed: %v", err)
	}
	if len(overview.Customers) != 1 {
		t.Fatalf("expected 1 customer runtime row, got %d", len(overview.Customers))
	}
	customerRow := overview.Customers[0]
	if customerRow.OnlineClients != 6 {
		t.Fatalf("expected 6 online clients, got %d", customerRow.OnlineClients)
	}
	if customerRow.HomeOnlineClients != 2 || customerRow.DedicatedOnlineClients != 4 {
		t.Fatalf("unexpected route split: home=%d dedicated=%d", customerRow.HomeOnlineClients, customerRow.DedicatedOnlineClients)
	}
	if customerRow.Traffic1H != 5300 || customerRow.Traffic24H != 5300 || customerRow.Traffic7D != 5300 {
		t.Fatalf("unexpected customer traffic windows: %+v", customerRow)
	}
	if customerRow.RealtimeBPS <= 0 {
		t.Fatalf("expected restart-safe realtime rate fallback, got %f", customerRow.RealtimeBPS)
	}

	if len(overview.Groups) != 1 {
		t.Fatalf("expected 1 group runtime row, got %d", len(overview.Groups))
	}
	groupRow := overview.Groups[0]
	if groupRow.GroupID != groupHead.ID || groupRow.Traffic1H != 4000 {
		t.Fatalf("unexpected group runtime row: %+v", groupRow)
	}

	if len(overview.Orders) != 2 {
		t.Fatalf("expected 2 order runtime rows, got %d", len(overview.Orders))
	}
	orderRows := map[uint]OrderRuntimeStat{}
	for _, row := range overview.Orders {
		orderRows[row.OrderID] = row
	}
	if orderRows[homeOrder.ID].Traffic1H != 1300 {
		t.Fatalf("unexpected home order runtime row: %+v", orderRows[homeOrder.ID])
	}
	if orderRows[childOrder.ID].Traffic1H != 4000 {
		t.Fatalf("unexpected dedicated order runtime row: %+v", orderRows[childOrder.ID])
	}
}

func TestRuntimeOverviewWarnsAndSkipsConflictedUsernames(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	now := time.Date(2026, 4, 7, 19, 0, 0, 0, time.UTC)

	customerA := model.Customer{Name: "A", Code: "A", Status: model.OrderStatusActive}
	customerB := model.Customer{Name: "B", Code: "B", Status: model.OrderStatusActive}
	if err := db.Create(&customerA).Error; err != nil {
		t.Fatalf("create customer A failed: %v", err)
	}
	if err := db.Create(&customerB).Error; err != nil {
		t.Fatalf("create customer B failed: %v", err)
	}

	orderA := model.Order{OrderNo: "OD-A-1", CustomerID: customerA.ID, Name: "A1", Mode: model.OrderModeAuto, Status: model.OrderStatusActive, Quantity: 1, Port: 23457, StartsAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	orderB := model.Order{OrderNo: "OD-B-1", CustomerID: customerB.ID, Name: "B1", Mode: model.OrderModeAuto, Status: model.OrderStatusActive, Quantity: 1, Port: 23458, StartsAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err := db.Create(&orderA).Error; err != nil {
		t.Fatalf("create order A failed: %v", err)
	}
	if err := db.Create(&orderB).Error; err != nil {
		t.Fatalf("create order B failed: %v", err)
	}
	if err := db.Create(&model.OrderItem{OrderID: orderA.ID, IP: "10.0.0.1", Port: 23457, Username: "dup-user", Password: "a", Managed: true, Status: model.OrderItemStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create order item A failed: %v", err)
	}
	if err := db.Create(&model.OrderItem{OrderID: orderB.ID, IP: "10.0.0.2", Port: 23458, Username: "dup-user", Password: "b", Managed: true, Status: model.OrderItemStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create order item B failed: %v", err)
	}

	svc := NewRuntimeStatsService(db, nil)
	svc.nowFn = func() time.Time { return now }
	svc.trafficProvider = func(context.Context) (map[string]int64, error) {
		return map[string]int64{
			"user>>>dup-user>>>traffic>>>uplink":   123,
			"user>>>dup-user>>>traffic>>>downlink": 456,
		}, nil
	}
	svc.onlineListProvider = func(context.Context) ([]string, error) {
		return []string{"user>>>dup-user>>>online"}, nil
	}
	svc.onlineCountsProvider = func(_ context.Context, statNames []string) (map[string]int64, error) {
		out := map[string]int64{}
		for _, statName := range statNames {
			out[statName] = 2
		}
		return out, nil
	}

	overview, err := svc.Overview(context.Background(), 30)
	if err != nil {
		t.Fatalf("overview failed: %v", err)
	}
	if len(overview.Warnings) == 0 {
		t.Fatal("expected duplicate username warning")
	}
	if len(overview.Customers) != 0 || len(overview.Orders) != 0 {
		t.Fatalf("expected conflicted usernames to be skipped from attributed stats, got customers=%d orders=%d", len(overview.Customers), len(overview.Orders))
	}
}
