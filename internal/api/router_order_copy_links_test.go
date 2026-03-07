package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"xraytool/internal/model"
	"xraytool/internal/service"
)

func TestCopyOrderLinksReturnsDedicatedLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAPICopyLinksTestDB(t)
	orderID := seedAPICopyLinksOrder(t, db)
	api := &API{orders: service.NewOrderService(db, nil, nil)}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", orderID)}}
	c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/orders/%d/copy-links", orderID), nil)

	api.copyOrderLinks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if body == "" || !containsAll(body, "socks://", "vless://", "vmess://", "ss://") {
		t.Fatalf("unexpected copy links body: %q", body)
	}
	if containsAll(body, "line.example.com:443:user01:pass01") {
		t.Fatalf("copy links should not include domain credential line: %q", body)
	}
}

func setupAPICopyLinksTestDB(t *testing.T) *gorm.DB {
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

func seedAPICopyLinksOrder(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	now := time.Now()
	customer := model.Customer{Name: "api-copy", Code: "api-copy", Status: model.OrderStatusActive}
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
	order := model.Order{CustomerID: customer.ID, Name: "child", Mode: model.OrderModeDedicated, Status: model.OrderStatusActive, Quantity: 1, Port: 443, StartsAt: now, ExpiresAt: now.Add(24 * time.Hour), DedicatedProtocol: model.DedicatedFeatureVless, DedicatedEntryID: &entry.ID, DedicatedInboundID: &inbound.ID, DedicatedIngressID: &ingress.ID, DedicatedEntry: &entry, DedicatedInbound: &inbound, DedicatedIngress: &ingress}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := model.OrderItem{OrderID: order.ID, IP: "127.0.0.1", Port: 443, Username: "user01", Password: "pass01", VmessUUID: "11111111-2222-3333-4444-555555555555", Managed: true, Status: model.OrderItemStatusActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item failed: %v", err)
	}
	return order.ID
}

func containsAll(body string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(body, part) {
			return false
		}
	}
	return true
}
