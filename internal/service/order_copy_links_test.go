package service

import (
	"strings"
	"testing"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
)

func TestCopyOrderLinkLinesForChildOrder(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, nil, nil)
	_, children := seedDedicatedCopyLinksTestGroup(t, db)

	text, err := svc.CopyOrderLinkLines(children[0])
	if err != nil {
		t.Fatalf("CopyOrderLinkLines failed: %v", err)
	}
	if strings.Contains(text, "line.example.com:443:user01:pass01") {
		t.Fatalf("copy links should not include domain credential line: %q", text)
	}
	for _, token := range []string{"socks://", "vless://", "vmess://", "ss://"} {
		if !strings.Contains(text, token) {
			t.Fatalf("expected %s in copied links, got: %q", token, text)
		}
	}
	if strings.Contains(text, "\n") {
		t.Fatalf("single child should produce single line, got: %q", text)
	}
}

func TestCopyOrderLinkLinesForGroupHeadReturnsMultipleChildren(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	svc := NewOrderService(db, nil, nil)
	headID, _ := seedDedicatedCopyLinksTestGroup(t, db)

	text, err := svc.CopyOrderLinkLines(headID)
	if err != nil {
		t.Fatalf("CopyOrderLinkLines failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 child lines, got %d: %q", len(lines), text)
	}
	for _, line := range lines {
		if strings.Contains(line, "line.example.com:443:user") {
			t.Fatalf("group copied links should not include domain credential line: %q", line)
		}
		if !strings.Contains(line, "vless://") {
			t.Fatalf("expected vless link in line: %q", line)
		}
	}
}

func seedDedicatedCopyLinksTestGroup(t *testing.T, db *gorm.DB) (uint, []uint) {
	t.Helper()
	now := time.Now()
	customer := model.Customer{Name: "copy-links", Code: "cl", Status: model.OrderStatusActive}
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
	head := model.Order{CustomerID: customer.ID, Name: "head", Mode: model.OrderModeDedicated, Status: model.OrderStatusActive, Quantity: 2, Port: 443, StartsAt: now, ExpiresAt: now.Add(24 * time.Hour), IsGroupHead: true, GroupID: 1, DedicatedProtocol: model.DedicatedFeatureVless, DedicatedEntryID: &entry.ID, DedicatedInboundID: &inbound.ID, DedicatedIngressID: &ingress.ID, DedicatedEntry: &entry, DedicatedInbound: &inbound, DedicatedIngress: &ingress}
	if err := db.Create(&head).Error; err != nil {
		t.Fatalf("create head failed: %v", err)
	}
	if err := db.Model(&model.Order{}).Where("id = ?", head.ID).Update("group_id", head.ID).Error; err != nil {
		t.Fatalf("update head group failed: %v", err)
	}
	childIDs := make([]uint, 0, 2)
	for idx := 0; idx < 2; idx++ {
		parentID := head.ID
		child := model.Order{CustomerID: customer.ID, GroupID: head.ID, ParentOrderID: &parentID, SequenceNo: idx + 1, Name: "child", Mode: model.OrderModeDedicated, Status: model.OrderStatusActive, Quantity: 1, Port: 443, StartsAt: now, ExpiresAt: now.Add(24 * time.Hour), DedicatedProtocol: model.DedicatedFeatureVless, DedicatedEntryID: &entry.ID, DedicatedInboundID: &inbound.ID, DedicatedIngressID: &ingress.ID, DedicatedEntry: &entry, DedicatedInbound: &inbound, DedicatedIngress: &ingress}
		if err := db.Create(&child).Error; err != nil {
			t.Fatalf("create child failed: %v", err)
		}
		item := model.OrderItem{OrderID: child.ID, IP: "127.0.0.1", Port: 443, Username: "user01", Password: "pass01", VmessUUID: "11111111-2222-3333-4444-555555555555", Managed: true, Status: model.OrderItemStatusActive, CreatedAt: now, UpdatedAt: now}
		if idx == 1 {
			item.Username = "user02"
			item.Password = "pass02"
			item.VmessUUID = "66666666-2222-3333-4444-555555555555"
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create item failed: %v", err)
		}
		childIDs = append(childIDs, child.ID)
	}
	return head.ID, childIDs
}
