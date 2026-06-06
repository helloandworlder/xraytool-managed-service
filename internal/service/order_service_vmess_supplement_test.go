package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
)

func TestCreateDedicatedVmessSupplementParsesLinksAndSocks5(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, &XrayManager{}, zap.NewNop())

	originalDispatch := dedicatedEgressAsyncDispatch
	defer func() { dedicatedEgressAsyncDispatch = originalDispatch }()
	dedicatedEgressAsyncDispatch = func(fn func()) {}

	customer := model.Customer{Name: "supp-customer", Code: "supp", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	inbound := model.DedicatedInbound{Name: "vmess-in", Protocol: model.DedicatedFeatureVmess, ListenPort: 24902, Enabled: true}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound failed: %v", err)
	}

	links := strings.Join([]string{
		"vmess://eyJ2IjoiMiIsInBzIjoi576O5Zu9LTEzMS4xMDMuMTI2LjUwIiwiYWRkIjoibGItc2hvcnR2aWRlby0xLmdvc2VhLmluIiwicG9ydCI6IjI0OTAyIiwiaWQiOiI1OTBmMDYxOS1jMTM1LTQyYmUtYTdiMC1mOWM3YjkwOWY2YzAiLCJhaWQiOiIwIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiIiwicGF0aCI6IiIsInRscyI6IiJ9",
		"vmess://eyJ2IjoiMiIsInBzIjoi576O5Zu9LTEzMS4xMDMuMTI2LjEzNSIsImFkZCI6ImxiLXNob3J0dmlkZW8tMS5nb3NlYS5pbiIsInBvcnQiOiIyNDkwMiIsImlkIjoiMDgzYTlhZTAtNzU0ZC00YTFkLWIzNjAtYjNlMjg1MzhlMjUxIiwiYWlkIjoiMCIsIm5ldCI6InRjcCIsInR5cGUiOiJub25lIiwiaG9zdCI6IiIsInBhdGgiOiIiLCJ0bHMiOiIifQ==",
	}, "\n")
	socks := strings.Join([]string{
		"131.103.126.50:34781:aHR9ESsm:IufiPeuCZr0o",
		"131.103.126.135:34781:byE13iP2:h0Rs6tHyxbjP",
	}, "\n")

	order, err := svc.CreateDedicatedVmessSupplement(context.Background(), DedicatedVmessSupplementInput{
		CustomerID:  customer.ID,
		Name:        "supplement",
		DurationDay: 29,
		VmessLinks:  links,
		Socks5Lines: socks,
	})
	if err != nil {
		t.Fatalf("CreateDedicatedVmessSupplement failed: %v", err)
	}
	if order.Quantity != 2 || order.Port != 24902 || order.DedicatedProtocol != model.DedicatedFeatureVmess {
		t.Fatalf("unexpected order: quantity=%d port=%d protocol=%s", order.Quantity, order.Port, order.DedicatedProtocol)
	}
	if order.ExpiresAt.Before(time.Now().Add(28 * 24 * time.Hour)) {
		t.Fatalf("expected duration-based future expiry, got %s", order.ExpiresAt)
	}
	children := []model.Order{}
	if err := db.Preload("Items").Where("parent_order_id = ?", order.ID).Order("sequence_no asc").Find(&children).Error; err != nil {
		t.Fatalf("load children failed: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	if children[0].Items[0].VmessUUID != "590f0619-c135-42be-a7b0-f9c7b909f6c0" {
		t.Fatalf("unexpected first uuid: %s", children[0].Items[0].VmessUUID)
	}
	if children[1].Items[0].ForwardAddress != "131.103.126.135" {
		t.Fatalf("unexpected second forward address: %s", children[1].Items[0].ForwardAddress)
	}
	ingress := model.DedicatedIngress{}
	if err := db.First(&ingress, "dedicated_inbound_id = ? and domain = ? and ingress_port = ?", inbound.ID, "lb-shortvideo-1.gosea.in", 24902).Error; err != nil {
		t.Fatalf("expected ingress to be created from vmess link: %v", err)
	}
}
