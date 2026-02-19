package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
	"golang.org/x/net/proxy"
	"gorm.io/gorm"
)

type OrderService struct {
	db   *gorm.DB
	xray *XrayManager
	log  *zap.Logger
}

type BatchActionResult struct {
	ID      uint   `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type BatchTestResult struct {
	ID      uint            `json:"id"`
	Success bool            `json:"success"`
	Result  map[uint]string `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type CreateOrderInput struct {
	CustomerID  uint   `json:"customer_id"`
	Name        string `json:"name"`
	Quantity    int    `json:"quantity"`
	DurationDay int    `json:"duration_day"`
	Mode        string `json:"mode"`
	Port        int    `json:"port"`
	ManualIPIDs []uint `json:"manual_ip_ids"`
}

type ImportPreviewRow struct {
	Raw          string `json:"raw"`
	IP           string `json:"ip"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	IsLocalIP    bool   `json:"is_local_ip"`
	PortOccupied bool   `json:"port_occupied"`
	Error        string `json:"error,omitempty"`
}

func NewOrderService(db *gorm.DB, xray *XrayManager, log *zap.Logger) *OrderService {
	return &OrderService{db: db, xray: xray, log: log}
}

func (s *OrderService) ListOrders() ([]model.Order, error) {
	var orders []model.Order
	err := s.db.Preload("Customer").Preload("Items").Order("id desc").Find(&orders).Error
	return orders, err
}

func (s *OrderService) GetOrder(orderID uint) (*model.Order, error) {
	var order model.Order
	err := s.db.Preload("Customer").Preload("Items").Preload("Items.Resources").First(&order, orderID).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, in CreateOrderInput) (*model.Order, error) {
	if in.CustomerID == 0 {
		return nil, errors.New("customer_id is required")
	}
	if in.Quantity <= 0 {
		return nil, errors.New("quantity must be > 0")
	}
	if in.DurationDay <= 0 {
		in.DurationDay = 30
	}
	if in.Mode == "" {
		in.Mode = model.OrderModeAuto
	}
	if in.Mode != model.OrderModeAuto && in.Mode != model.OrderModeManual {
		return nil, errors.New("mode must be auto or manual")
	}

	port := in.Port
	if port <= 0 {
		p, err := s.defaultPort()
		if err != nil {
			return nil, err
		}
		port = p
	}

	selectedIPs, err := s.allocateIPs(in.CustomerID, in.Quantity, in.Mode, in.ManualIPIDs)
	if err != nil {
		return nil, err
	}
	if len(selectedIPs) != in.Quantity {
		return nil, fmt.Errorf("not enough IPs, expect %d got %d", in.Quantity, len(selectedIPs))
	}

	now := time.Now()
	order := &model.Order{
		CustomerID: in.CustomerID,
		Name:       in.Name,
		Mode:       in.Mode,
		Status:     model.OrderStatusActive,
		Quantity:   in.Quantity,
		Port:       port,
		StartsAt:   now,
		ExpiresAt:  now.Add(time.Duration(in.DurationDay) * 24 * time.Hour),
	}
	if strings.TrimSpace(order.Name) == "" {
		order.Name = fmt.Sprintf("Order-%d-%s", in.CustomerID, now.Format("20060102150405"))
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for _, ip := range selectedIPs {
			item := model.OrderItem{
				OrderID:   order.ID,
				HostIPID:  &ip.ID,
				IP:        ip.IP,
				Port:      port,
				Username:  randomString(8),
				Password:  randomString(12),
				Managed:   true,
				Status:    model.OrderItemStatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.SyncOrderRuntime(ctx, order.ID); err != nil {
		s.log.Warn("sync runtime after create failed", zap.Error(err), zap.Uint("order_id", order.ID))
	}

	if err := s.db.Preload("Customer").Preload("Items").First(order, order.ID).Error; err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) SyncOrderRuntime(ctx context.Context, orderID uint) error {
	var order model.Order
	if err := s.db.Preload("Items").First(&order, orderID).Error; err != nil {
		return err
	}

	return s.xray.ApplyOrFallback(ctx, func(ctx context.Context) error {
		for _, item := range order.Items {
			if !item.Managed || item.Status != model.OrderItemStatusActive {
				continue
			}
			resource, err := s.xray.ApplyOrderItem(ctx, item, InboundTag(item.Port))
			if err != nil && !isAlreadyExists(err) {
				return err
			}
			resource.OrderItemID = item.ID
			if err := s.db.Where("order_item_id = ?", item.ID).Assign(resource).FirstOrCreate(&model.XrayResource{}).Error; err != nil {
				return err
			}
		}
		ports := map[int]struct{}{}
		for _, item := range order.Items {
			if item.Managed {
				ports[item.Port] = struct{}{}
			}
		}
		for port := range ports {
			accounts, err := s.activeAccountsForPort(port)
			if err != nil {
				return err
			}
			if err := s.xray.RebuildInboundForPort(ctx, port, accounts); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *OrderService) DeactivateOrder(ctx context.Context, orderID uint, status string) error {
	if status == "" {
		status = model.OrderStatusDisabled
	}
	itemStatus := model.OrderItemStatusDisabled
	if status == model.OrderStatusExpired {
		itemStatus = model.OrderItemStatusExpired
	}
	var order model.Order
	if err := s.db.Preload("Items").First(&order, orderID).Error; err != nil {
		return err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).Where("id = ?", orderID).Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.OrderItem{}).Where("order_id = ?", orderID).Updates(map[string]interface{}{
			"status":     itemStatus,
			"updated_at": time.Now(),
		}).Error
	}); err != nil {
		return err
	}

	return s.xray.ApplyOrFallback(ctx, func(ctx context.Context) error {
		ports := map[int]struct{}{}
		for _, item := range order.Items {
			ports[item.Port] = struct{}{}
			if !item.Managed {
				continue
			}
			if err := s.xray.RemoveItemResource(ctx, model.XrayResource{
				OrderItemID: item.ID,
				OutboundTag: OutboundTag(item.ID),
				RuleTag:     RuleTag(item.ID),
			}); err != nil && !isNotFoundErr(err) {
				return err
			}
		}
		for port := range ports {
			accounts, err := s.activeAccountsForPort(port)
			if err != nil {
				return err
			}
			if err := s.xray.RebuildInboundForPort(ctx, port, accounts); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *OrderService) BatchDeactivate(ctx context.Context, orderIDs []uint, status string) []BatchActionResult {
	out := make([]BatchActionResult, 0, len(orderIDs))
	for _, id := range orderIDs {
		err := s.DeactivateOrder(ctx, id, status)
		entry := BatchActionResult{ID: id, Success: err == nil}
		if err != nil {
			entry.Error = err.Error()
		}
		out = append(out, entry)
	}
	return out
}

func (s *OrderService) RenewOrder(ctx context.Context, orderID uint, moreDays int) error {
	if moreDays <= 0 {
		moreDays = 30
	}
	var order model.Order
	if err := s.db.First(&order, orderID).Error; err != nil {
		return err
	}
	base := order.ExpiresAt
	if base.Before(time.Now()) {
		base = time.Now()
	}
	newExpires := base.Add(time.Duration(moreDays) * 24 * time.Hour)
	if err := s.db.Model(&model.Order{}).Where("id = ?", orderID).Updates(map[string]interface{}{
		"status":              model.OrderStatusActive,
		"expires_at":          newExpires,
		"notify_one_day_sent": false,
		"notify_expired_sent": false,
		"updated_at":          time.Now(),
	}).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.OrderItem{}).Where("order_id = ?", orderID).Update("status", model.OrderItemStatusActive).Error; err != nil {
		return err
	}
	return s.SyncOrderRuntime(ctx, orderID)
}

func (s *OrderService) BatchRenew(ctx context.Context, orderIDs []uint, moreDays int) []BatchActionResult {
	out := make([]BatchActionResult, 0, len(orderIDs))
	for _, id := range orderIDs {
		err := s.RenewOrder(ctx, id, moreDays)
		entry := BatchActionResult{ID: id, Success: err == nil}
		if err != nil {
			entry.Error = err.Error()
		}
		out = append(out, entry)
	}
	return out
}

func (s *OrderService) BatchResync(ctx context.Context, orderIDs []uint) []BatchActionResult {
	out := make([]BatchActionResult, 0, len(orderIDs))
	for _, id := range orderIDs {
		err := s.SyncOrderRuntime(ctx, id)
		entry := BatchActionResult{ID: id, Success: err == nil}
		if err != nil {
			entry.Error = err.Error()
		}
		out = append(out, entry)
	}
	return out
}

func (s *OrderService) BatchTest(orderIDs []uint) []BatchTestResult {
	out := make([]BatchTestResult, 0, len(orderIDs))
	for _, id := range orderIDs {
		res, err := s.TestOrder(id)
		entry := BatchTestResult{ID: id, Success: err == nil, Result: res}
		if err != nil {
			entry.Error = err.Error()
		}
		out = append(out, entry)
	}
	return out
}

func (s *OrderService) BatchExport(orderIDs []uint) (string, error) {
	parts := make([]string, 0, len(orderIDs)*2)
	for _, id := range orderIDs {
		lines, err := s.ExportOrderLines(id)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("# Order %d", id))
		parts = append(parts, lines)
	}
	return strings.Join(parts, "\n"), nil
}

func (s *OrderService) ExportOrderLines(orderID uint) (string, error) {
	var items []model.OrderItem
	if err := s.db.Where("order_id = ?", orderID).Order("id asc").Find(&items).Error; err != nil {
		return "", err
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s:%d:%s:%s", item.IP, item.Port, item.Username, item.Password))
	}
	return strings.Join(lines, "\n"), nil
}

func (s *OrderService) TestOrder(orderID uint) (map[uint]string, error) {
	var items []model.OrderItem
	if err := s.db.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, err
	}
	out := make(map[uint]string, len(items))
	for _, item := range items {
		if !item.Managed {
			out[item.ID] = "unmanaged"
			continue
		}
		socksAddr := fmt.Sprintf("%s:%d", item.IP, item.Port)
		dialer, err := proxy.SOCKS5("tcp", socksAddr, &proxy.Auth{User: item.Username, Password: item.Password}, proxy.Direct)
		if err != nil {
			out[item.ID] = "dialer error"
			continue
		}
		httpTransport := &http.Transport{Dial: dialer.Dial}
		client := &http.Client{Timeout: 6 * time.Second, Transport: httpTransport}
		resp, err := client.Get("https://api.ipify.org")
		if err != nil {
			out[item.ID] = "failed"
			continue
		}
		body := make([]byte, 128)
		n, _ := resp.Body.Read(body)
		_ = resp.Body.Close()
		out[item.ID] = strings.TrimSpace(string(body[:n]))
	}
	return out, nil
}

func (s *OrderService) PreviewImport(lines string) ([]ImportPreviewRow, error) {
	hostIPs, err := s.hostIPSet()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(lines))
	rows := make([]ImportPreviewRow, 0)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		row := ImportPreviewRow{Raw: raw}
		parts := strings.Split(raw, ":")
		if len(parts) != 4 {
			row.Error = "format must be ip:port:user:pass"
			rows = append(rows, row)
			continue
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil || port <= 0 || port > 65535 {
			row.Error = "invalid port"
			rows = append(rows, row)
			continue
		}
		row.IP = parts[0]
		row.Port = port
		row.Username = parts[2]
		row.Password = parts[3]
		_, row.IsLocalIP = hostIPs[row.IP]
		if row.IsLocalIP {
			occupied, _ := ProbePort(row.IP, row.Port)
			row.PortOccupied = occupied
		}
		rows = append(rows, row)
	}
	return rows, scanner.Err()
}

func (s *OrderService) ImportOrder(ctx context.Context, customerID uint, orderName string, expiresAt time.Time, rows []ImportPreviewRow) (*model.Order, error) {
	if customerID == 0 {
		return nil, errors.New("customer_id required")
	}
	validRows := make([]ImportPreviewRow, 0, len(rows))
	for _, row := range rows {
		if row.Error == "" {
			validRows = append(validRows, row)
		}
	}
	if len(validRows) == 0 {
		return nil, errors.New("no valid rows")
	}
	if orderName == "" {
		orderName = fmt.Sprintf("Imported-%s", time.Now().Format("20060102150405"))
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}

	order := &model.Order{
		CustomerID: customerID,
		Name:       orderName,
		Mode:       model.OrderModeImport,
		Status:     model.OrderStatusActive,
		Quantity:   len(validRows),
		Port:       validRows[0].Port,
		StartsAt:   time.Now(),
		ExpiresAt:  expiresAt,
	}

	hostRows := []model.HostIP{}
	if err := s.db.Find(&hostRows).Error; err != nil {
		return nil, err
	}
	hostByIP := map[string]model.HostIP{}
	for _, h := range hostRows {
		hostByIP[h.IP] = h
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for _, row := range validRows {
			item := model.OrderItem{
				OrderID:   order.ID,
				IP:        row.IP,
				Port:      row.Port,
				Username:  row.Username,
				Password:  row.Password,
				Managed:   row.IsLocalIP,
				Status:    model.OrderItemStatusActive,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if host, ok := hostByIP[row.IP]; ok {
				item.HostIPID = &host.ID
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.SyncOrderRuntime(ctx, order.ID); err != nil {
		s.log.Warn("sync runtime after import failed", zap.Error(err), zap.Uint("order_id", order.ID))
	}
	if err := s.db.Preload("Customer").Preload("Items").First(order, order.ID).Error; err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) allocateIPs(customerID uint, quantity int, mode string, manualIDs []uint) ([]model.HostIP, error) {
	if mode == model.OrderModeManual {
		if len(manualIDs) < quantity {
			return nil, errors.New("manual_ip_ids insufficient")
		}
		var rows []model.HostIP
		if err := s.db.Where("id in ? and enabled = 1", manualIDs[:quantity]).Find(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) != quantity {
			return nil, errors.New("some manual ips are invalid or disabled")
		}
		return rows, nil
	}

	var all []model.HostIP
	if err := s.db.Where("enabled = 1 and is_local = 1 and is_public = 1").Order("ip asc").Find(&all).Error; err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, errors.New("no enabled host ips")
	}

	usedByCustomer := map[string]struct{}{}
	var ips []struct{ IP string }
	if err := s.db.Table("order_items oi").
		Select("oi.ip").
		Joins("join orders o on o.id = oi.order_id").
		Where("o.customer_id = ? and o.status = ? and o.expires_at > ? and oi.status = ?", customerID, model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
		Scan(&ips).Error; err != nil {
		return nil, err
	}
	for _, v := range ips {
		usedByCustomer[v.IP] = struct{}{}
	}

	usage := map[string]int64{}
	type usageRow struct {
		IP    string
		Count int64
	}
	var usageRows []usageRow
	if err := s.db.Table("order_items oi").
		Select("oi.ip as ip, count(1) as count").
		Joins("join orders o on o.id = oi.order_id").
		Where("o.status = ? and o.expires_at > ? and oi.status = ?", model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
		Group("oi.ip").Scan(&usageRows).Error; err != nil {
		return nil, err
	}
	for _, u := range usageRows {
		usage[u.IP] = u.Count
	}

	candidates := make([]model.HostIP, 0, len(all))
	for _, ip := range all {
		if _, exists := usedByCustomer[ip.IP]; exists {
			continue
		}
		candidates = append(candidates, ip)
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := usage[candidates[i].IP]
		right := usage[candidates[j].IP]
		if left == right {
			return candidates[i].IP < candidates[j].IP
		}
		return left < right
	})

	if len(candidates) < quantity {
		return nil, fmt.Errorf("available IPs (%d) less than quantity (%d)", len(candidates), quantity)
	}
	return candidates[:quantity], nil
}

func (s *OrderService) activeAccountsForPort(port int) (map[string]string, error) {
	rows := []struct {
		Username string
		Password string
	}{}
	err := s.db.Table("order_items oi").
		Select("oi.username, oi.password").
		Joins("join orders o on o.id = oi.order_id").
		Where("oi.port = ? and oi.managed = 1 and oi.status = ? and o.status = ? and o.expires_at > ?", port, model.OrderItemStatusActive, model.OrderStatusActive, time.Now()).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	accounts := map[string]string{}
	for _, row := range rows {
		accounts[row.Username] = row.Password
	}
	return accounts, nil
}

func (s *OrderService) defaultPort() (int, error) {
	var row model.Setting
	if err := s.db.First(&row, "key = ?", "default_inbound_port").Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 23457, nil
		}
		return 0, err
	}
	p, err := strconv.Atoi(row.Value)
	if err != nil || p <= 0 {
		return 23457, nil
	}
	return p, nil
}

func (s *OrderService) hostIPSet() (map[string]uint, error) {
	var rows []model.HostIP
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	set := map[string]uint{}
	for _, row := range rows {
		set[row.IP] = row.ID
	}
	return set, nil
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists")
}
