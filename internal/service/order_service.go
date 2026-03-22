package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
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
	CustomerID                    uint      `json:"customer_id"`
	Name                          string    `json:"name"`
	Quantity                      int       `json:"quantity"`
	DurationDay                   int       `json:"duration_day"`
	ExpiresAt                     time.Time `json:"expires_at"`
	Mode                          string    `json:"mode"`
	Port                          int       `json:"port"`
	ManualIPIDs                   []uint    `json:"manual_ip_ids"`
	ResidentialCredentialMode     string    `json:"residential_credential_mode"`
	ResidentialCredentialStrategy string    `json:"residential_credential_strategy"`
	ResidentialCredentialLines    string    `json:"residential_credential_lines"`
	ForwardOutboundIDs            []uint    `json:"forward_outbound_ids"`
	DedicatedEntryID              uint      `json:"dedicated_entry_id"`
	DedicatedInboundID            uint      `json:"dedicated_inbound_id"`
	DedicatedIngressID            uint      `json:"dedicated_ingress_id"`
	DedicatedProtocol             string    `json:"dedicated_protocol"`
	DedicatedEgressLines          string    `json:"dedicated_egress_lines"`
}

type UpdateOrderInput struct {
	Name                           string    `json:"name"`
	Quantity                       int       `json:"quantity"`
	Port                           int       `json:"port"`
	ExpiresAt                      time.Time `json:"expires_at"`
	ManualIPIDs                    []uint    `json:"manual_ip_ids"`
	ResidentialCredentialMode      string    `json:"residential_credential_mode"`
	ResidentialCredentialStrategy  string    `json:"residential_credential_strategy"`
	ResidentialCredentialLines     string    `json:"residential_credential_lines"`
	ForwardOutboundIDs             []uint    `json:"forward_outbound_ids"`
	DedicatedEntryID               uint      `json:"dedicated_entry_id"`
	DedicatedInboundID             uint      `json:"dedicated_inbound_id"`
	DedicatedIngressID             uint      `json:"dedicated_ingress_id"`
	DedicatedProtocol              string    `json:"dedicated_protocol"`
	DedicatedEgressLines           string    `json:"dedicated_egress_lines"`
	DedicatedCredentialLines       string    `json:"dedicated_credential_lines"`
	RegenerateDedicatedCredentials bool      `json:"regenerate_dedicated_credentials"`
}

type AllocationPreview struct {
	PoolSize       int `json:"pool_size"`
	UsedByCustomer int `json:"used_by_customer"`
	Available      int `json:"available"`
}

type ExportOrderOptions struct {
	Count                int
	Shuffle              bool
	ResidentialTXTLayout string
}

const (
	ResidentialCredentialModeRandom      = "random"
	ResidentialCredentialModeCustom      = "custom"
	ResidentialCredentialStrategyPerLine = "per_line"
	ResidentialCredentialStrategyShared  = "shared"
)

type ResidentialCredentialLine struct {
	Username string
	Password string
}

type residentialCredentialAssignment struct {
	IP       string
	Username string
	Password string
}

type TestOrderStreamEvent struct {
	Type          string `json:"type"`
	ItemID        uint   `json:"item_id,omitempty"`
	Status        string `json:"status,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Total         int    `json:"total,omitempty"`
	Sampled       int    `json:"sampled,omitempty"`
	SamplePercent int    `json:"sample_percent,omitempty"`
	SuccessCount  int    `json:"success_count,omitempty"`
	FailureCount  int    `json:"failure_count,omitempty"`
	Error         string `json:"error,omitempty"`
}

type ImportPreviewRow struct {
	Raw          string `json:"raw"`
	SourceFile   string `json:"source_file,omitempty"`
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
	err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").Preload("Items").Order("id desc").Find(&orders).Error
	return orders, err
}

func (s *OrderService) GetOrder(orderID uint) (*model.Order, error) {
	var order model.Order
	err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").Preload("Items").Preload("Items.Resources").First(&order, orderID).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *OrderService) AllocationPreview(customerID uint, excludeOrderID uint) (AllocationPreview, error) {
	if customerID == 0 {
		return AllocationPreview{}, errors.New("customer_id is required")
	}
	pool, err := s.usableIPPool()
	if err != nil {
		return AllocationPreview{}, err
	}
	usedByCustomer, err := s.customerUsedIPSet(customerID, excludeOrderID)
	if err != nil {
		return AllocationPreview{}, err
	}
	available := len(pool) - len(usedByCustomer)
	if available < 0 {
		available = 0
	}
	return AllocationPreview{
		PoolSize:       len(pool),
		UsedByCustomer: len(usedByCustomer),
		Available:      available,
	}, nil
}

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint, in UpdateOrderInput) (*model.Order, error) {
	var order model.Order
	if err := s.db.Preload("Items").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").First(&order, orderID).Error; err != nil {
		return nil, err
	}
	if order.IsGroupHead {
		if err := s.updateOrderGroup(ctx, order, in); err != nil {
			return nil, err
		}
		updated, err := s.GetOrder(order.ID)
		if err != nil {
			return nil, err
		}
		return updated, nil
	}

	targetName := strings.TrimSpace(order.Name)
	if strings.TrimSpace(in.Name) != "" {
		targetName = strings.TrimSpace(in.Name)
	}
	targetPort := order.Port
	if in.Port > 0 {
		targetPort = in.Port
	}
	targetQuantity := order.Quantity
	targetDedicatedProtocol := strings.TrimSpace(order.DedicatedProtocol)
	if targetDedicatedProtocol == "" {
		targetDedicatedProtocol = model.DedicatedFeatureMixed
	}
	targetDedicatedEntryID := order.DedicatedEntryID
	targetDedicatedInboundID := order.DedicatedInboundID
	targetDedicatedIngressID := order.DedicatedIngressID
	targetForwardOutboundIDs := uniqueUintIDs(in.ForwardOutboundIDs)
	residentialCredentialMode := normalizeResidentialCredentialMode(in.ResidentialCredentialMode)
	residentialCredentialStrategy := normalizeResidentialCredentialStrategy(in.ResidentialCredentialStrategy)
	residentialCredentials := []ResidentialCredentialLine{}
	if order.Mode != model.OrderModeDedicated && (residentialCredentialMode == ResidentialCredentialModeCustom || strings.TrimSpace(in.ResidentialCredentialLines) != "") {
		residentialCredentialMode = ResidentialCredentialModeCustom
		var err error
		residentialCredentials, err = parseResidentialCredentialLines(in.ResidentialCredentialLines)
		if err != nil {
			return nil, err
		}
	}
	if order.Mode == model.OrderModeForward {
		if len(targetForwardOutboundIDs) == 0 {
			for _, item := range order.Items {
				if item.SocksOutboundID != nil && *item.SocksOutboundID > 0 {
					targetForwardOutboundIDs = append(targetForwardOutboundIDs, *item.SocksOutboundID)
				}
			}
			targetForwardOutboundIDs = uniqueUintIDs(targetForwardOutboundIDs)
		}
		targetQuantity = len(targetForwardOutboundIDs)
	} else if in.Quantity > 0 {
		targetQuantity = in.Quantity
	}
	if order.Mode == model.OrderModeDedicated {
		targetQuantity = len(order.Items)
		if strings.TrimSpace(in.DedicatedProtocol) != "" {
			normalized, err := normalizeDedicatedProtocol(in.DedicatedProtocol)
			if err != nil {
				return nil, err
			}
			targetDedicatedProtocol = normalized
		}
		if in.DedicatedEntryID > 0 {
			targetDedicatedEntryID = &in.DedicatedEntryID
		}
		if in.DedicatedInboundID > 0 {
			targetDedicatedInboundID = &in.DedicatedInboundID
		}
		if in.DedicatedIngressID > 0 {
			targetDedicatedIngressID = &in.DedicatedIngressID
		}
		resolvedEntryID, resolvedInboundID, resolvedIngressID, resolvedPort, resolveErr := s.resolveDedicatedBindingForUpdateTx(s.db, targetDedicatedProtocol, targetDedicatedEntryID, targetDedicatedInboundID, targetDedicatedIngressID)
		if resolveErr != nil {
			return nil, resolveErr
		}
		targetDedicatedEntryID = resolvedEntryID
		targetDedicatedInboundID = resolvedInboundID
		targetDedicatedIngressID = resolvedIngressID
		targetPort = resolvedPort
	}
	targetExpiresAt := order.ExpiresAt
	if !in.ExpiresAt.IsZero() {
		targetExpiresAt = in.ExpiresAt
	}

	if targetPort <= 0 || targetPort > 65535 {
		return nil, errors.New("invalid port")
	}
	if targetQuantity <= 0 {
		return nil, errors.New("quantity must be > 0")
	}
	if order.Mode == model.OrderModeForward && len(targetForwardOutboundIDs) == 0 {
		return nil, errors.New("forward_outbound_ids is required for forward mode")
	}
	if order.Mode == model.OrderModeDedicated && in.Quantity > 0 && in.Quantity != len(order.Items) {
		return nil, errors.New("dedicated order quantity is fixed to item count")
	}
	if order.Mode != model.OrderModeDedicated && len(residentialCredentials) > 0 {
		expandedCredentials, err := expandResidentialCredentialLines(residentialCredentials, residentialCredentialStrategy, targetQuantity)
		if err != nil {
			return nil, err
		}
		residentialCredentials = expandedCredentials
	}
	if order.Mode == model.OrderModeImport && targetQuantity > len(order.Items) {
		return nil, errors.New("import order quantity cannot be increased")
	}
	if targetPort != order.Port && order.Mode != model.OrderModeDedicated {
		if err := s.ensurePortReadyForManaged(targetPort); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if order.Mode == model.OrderModeForward {
			if err := s.syncForwardOrderItemsTx(tx, order, targetForwardOutboundIDs, targetPort, targetExpiresAt, now); err != nil {
				return err
			}
		} else {
			items := append([]model.OrderItem(nil), order.Items...)
			if targetQuantity < len(items) {
				sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
				removeItems := items[:len(items)-targetQuantity]
				removeIDs := make([]uint, 0, len(removeItems))
				for _, item := range removeItems {
					removeIDs = append(removeIDs, item.ID)
				}
				if len(removeIDs) > 0 {
					if err := tx.Where("order_item_id in ?", removeIDs).Delete(&model.XrayResource{}).Error; err != nil {
						return err
					}
					if err := tx.Where("id in ?", removeIDs).Delete(&model.OrderItem{}).Error; err != nil {
						return err
					}
				}
			}
			if targetQuantity > len(items) {
				diff := targetQuantity - len(items)
				mode := order.Mode
				manualIDs := in.ManualIPIDs
				ipMode := mode
				if mode == model.OrderModeManual && len(manualIDs) < diff {
					ipMode = model.OrderModeAuto
					manualIDs = nil
				}
				if ipMode != model.OrderModeAuto && ipMode != model.OrderModeManual {
					ipMode = model.OrderModeAuto
				}
				selectedIPs, err := s.allocateIPs(order.CustomerID, diff, ipMode, manualIDs, order.ID)
				if err != nil {
					return err
				}
				for _, ip := range selectedIPs {
					username, err := s.nextAvailableResidentialUsernameTx(tx, ip.IP, order.ID)
					if err != nil {
						return err
					}
					item := model.OrderItem{
						OrderID:      order.ID,
						HostIPID:     &ip.ID,
						IP:           ip.IP,
						Port:         targetPort,
						Username:     username,
						Password:     randomString(12),
						OutboundType: model.OutboundTypeDirect,
						Managed:      true,
						Status:       model.OrderItemStatusActive,
						CreatedAt:    now,
						UpdatedAt:    now,
					}
					if err := tx.Create(&item).Error; err != nil {
						return err
					}
				}
			}

			itemUpdates := map[string]interface{}{
				"updated_at": now,
			}
			if targetPort != order.Port {
				itemUpdates["port"] = targetPort
			}
			if targetExpiresAt.After(now) {
				itemUpdates["status"] = model.OrderItemStatusActive
			}
			if err := tx.Model(&model.OrderItem{}).Where("order_id = ?", order.ID).Updates(itemUpdates).Error; err != nil {
				return err
			}
			if len(residentialCredentials) > 0 {
				currentItems := []model.OrderItem{}
				if err := tx.Where("order_id = ?", order.ID).Order("id asc").Find(&currentItems).Error; err != nil {
					return err
				}
				if len(currentItems) != len(residentialCredentials) {
					return fmt.Errorf("credential count %d not equal order items %d", len(residentialCredentials), len(currentItems))
				}
				assignments, err := residentialAssignmentsFromOrderItems(currentItems, residentialCredentials)
				if err != nil {
					return err
				}
				if err := s.ensureResidentialCredentialAssignmentsAvailableTx(tx, assignments, order.ID); err != nil {
					return err
				}
				for i, item := range currentItems {
					cred := residentialCredentials[i]
					if err := tx.Model(&model.OrderItem{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
						"username":   cred.Username,
						"password":   cred.Password,
						"updated_at": now,
					}).Error; err != nil {
						return err
					}
				}
			}
		}

		orderStatus := model.OrderStatusActive
		if !targetExpiresAt.After(now) {
			orderStatus = model.OrderStatusExpired
		}
		orderUpdates := map[string]interface{}{
			"name":                targetName,
			"quantity":            targetQuantity,
			"port":                targetPort,
			"expires_at":          targetExpiresAt,
			"status":              orderStatus,
			"notify_one_day_sent": false,
			"notify_expired_sent": false,
			"updated_at":          now,
		}
		if order.Mode == model.OrderModeDedicated {
			orderUpdates["dedicated_protocol"] = targetDedicatedProtocol
			if targetDedicatedEntryID != nil {
				orderUpdates["dedicated_entry_id"] = *targetDedicatedEntryID
			}
			if targetDedicatedInboundID != nil {
				orderUpdates["dedicated_inbound_id"] = *targetDedicatedInboundID
			}
			if targetDedicatedIngressID != nil {
				orderUpdates["dedicated_ingress_id"] = *targetDedicatedIngressID
			}
		}
		return tx.Model(&model.Order{}).Where("id = ?", order.ID).Updates(orderUpdates).Error
	}); err != nil {
		return nil, err
	}

	if !targetExpiresAt.After(now) {
		if err := s.DeactivateOrder(ctx, order.ID, model.OrderStatusExpired); err != nil {
			return nil, err
		}
	} else {
		if err := s.SyncOrderRuntime(ctx, order.ID); err != nil {
			s.log.Warn("sync runtime after update failed", zap.Error(err), zap.Uint("order_id", order.ID))
		}
	}

	updated, err := s.GetOrder(order.ID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, in CreateOrderInput) (*model.Order, error) {
	if in.CustomerID == 0 {
		return nil, errors.New("customer_id is required")
	}
	if in.DurationDay <= 0 && in.ExpiresAt.IsZero() {
		in.DurationDay = 30
	}
	if in.Mode == "" {
		in.Mode = model.OrderModeAuto
	}
	if in.Mode != model.OrderModeAuto && in.Mode != model.OrderModeManual && in.Mode != model.OrderModeDedicated {
		return nil, errors.New("mode must be auto/manual/dedicated")
	}
	if in.Mode == model.OrderModeForward {
		return nil, errors.New("forward mode is deprecated, use auto/manual for residential orders")
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(in.DurationDay) * 24 * time.Hour)
	if !in.ExpiresAt.IsZero() {
		if !in.ExpiresAt.After(now) {
			return nil, errors.New("expires_at must be in the future")
		}
		expiresAt = in.ExpiresAt
	}

	if in.Mode == model.OrderModeDedicated {
		return s.createDedicatedOrder(ctx, in, now, expiresAt)
	}

	if in.Quantity <= 0 {
		return nil, errors.New("quantity must be > 0")
	}
	residentialCredentialMode := normalizeResidentialCredentialMode(in.ResidentialCredentialMode)
	residentialCredentialStrategy := normalizeResidentialCredentialStrategy(in.ResidentialCredentialStrategy)
	residentialCredentials := []ResidentialCredentialLine{}
	if residentialCredentialMode == ResidentialCredentialModeCustom {
		var err error
		residentialCredentials, err = parseResidentialCredentialLines(in.ResidentialCredentialLines)
		if err != nil {
			return nil, err
		}
		residentialCredentials, err = expandResidentialCredentialLines(residentialCredentials, residentialCredentialStrategy, in.Quantity)
		if err != nil {
			return nil, err
		}
	}

	port := in.Port
	if port <= 0 {
		p, err := s.defaultPort()
		if err != nil {
			return nil, err
		}
		port = p
	}
	if err := s.ensurePortReadyForManaged(port); err != nil {
		return nil, err
	}

	ipMode := in.Mode
	if in.Mode == model.OrderModeForward {
		ipMode = model.OrderModeAuto
	}
	selectedIPs, err := s.allocateIPs(in.CustomerID, in.Quantity, ipMode, in.ManualIPIDs, 0)
	if err != nil {
		return nil, err
	}
	if len(selectedIPs) != in.Quantity {
		return nil, fmt.Errorf("not enough IPs, expect %d got %d", in.Quantity, len(selectedIPs))
	}

	selectedOutbounds := []model.SocksOutbound{}
	if in.Mode == model.OrderModeForward {
		selectedOutbounds, err = s.loadForwardOutboundsByIDs(in.ForwardOutboundIDs, true)
		if err != nil {
			return nil, err
		}
	}
	order := &model.Order{
		CustomerID: in.CustomerID,
		Name:       in.Name,
		Mode:       in.Mode,
		Status:     model.OrderStatusActive,
		Quantity:   in.Quantity,
		Port:       port,
		StartsAt:   now,
		ExpiresAt:  expiresAt,
	}
	if strings.TrimSpace(order.Name) == "" {
		prefix := s.residentialNamePrefix()
		order.Name = fmt.Sprintf("%s-%d-%s", prefix, in.CustomerID, now.Format("20060102150405"))
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if len(residentialCredentials) > 0 {
			assignments, err := residentialAssignmentsFromHostIPs(selectedIPs, residentialCredentials)
			if err != nil {
				return err
			}
			if err := s.ensureResidentialCredentialAssignmentsAvailableTx(tx, assignments, 0); err != nil {
				return err
			}
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		orderNo := buildOrderNo(order.CreatedAt, order.ID)
		if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{"order_no": orderNo, "updated_at": now}).Error; err != nil {
			return err
		}
		order.OrderNo = orderNo
		for i, ip := range selectedIPs {
			username := ""
			password := randomString(12)
			if len(residentialCredentials) > 0 {
				username = residentialCredentials[i].Username
				password = residentialCredentials[i].Password
			} else {
				username, err = s.nextAvailableResidentialUsernameTx(tx, ip.IP, 0)
				if err != nil {
					return err
				}
			}
			item := model.OrderItem{
				OrderID:      order.ID,
				HostIPID:     &ip.ID,
				IP:           ip.IP,
				Port:         port,
				Username:     username,
				Password:     password,
				OutboundType: model.OutboundTypeDirect,
				Managed:      true,
				Status:       model.OrderItemStatusActive,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if in.Mode == model.OrderModeForward {
				outbound := selectedOutbounds[i]
				user, userErr := s.resolveForwardItemUsername(tx, outbound, now, 0)
				if userErr != nil {
					return userErr
				}
				item.Username = user
				item.OutboundType = model.OutboundTypeSocks5
				item.SocksOutboundID = &outbound.ID
				item.ForwardAddress = outbound.Address
				item.ForwardPort = outbound.Port
				item.ForwardUsername = outbound.Username
				item.ForwardPassword = outbound.Password
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

	if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").Preload("Items").First(order, order.ID).Error; err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) SyncOrderRuntime(ctx context.Context, orderID uint) error {
	var order model.Order
	if err := s.db.Preload("Items").First(&order, orderID).Error; err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
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
	if order.IsGroupHead {
		return s.deactivateOrderGroup(ctx, order, status, itemStatus)
	}
	if order.Mode == model.OrderModeDedicated || order.ParentOrderID != nil {
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
		return s.rebuildManagedRuntime(ctx)
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

	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) ActivateOrder(ctx context.Context, orderID uint) error {
	var order model.Order
	if err := s.db.Preload("Items").First(&order, orderID).Error; err != nil {
		return err
	}
	if order.Status == model.OrderStatusActive {
		return nil
	}
	if order.Status != model.OrderStatusDisabled {
		return fmt.Errorf("only disabled order can be activated, current status: %s", order.Status)
	}
	now := time.Now()
	if !order.ExpiresAt.After(now) {
		return errors.New("order already expired, please renew first")
	}

	if order.IsGroupHead {
		var expiredCount int64
		if err := s.db.Model(&model.Order{}).Where("(id = ? or parent_order_id = ?) and expires_at <= ?", order.ID, order.ID, now).Count(&expiredCount).Error; err != nil {
			return err
		}
		if expiredCount > 0 {
			return errors.New("group contains expired child orders, please renew first")
		}
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.Order{}).Where("id = ? or parent_order_id = ?", order.ID, order.ID).Updates(map[string]interface{}{
				"status":              model.OrderStatusActive,
				"notify_one_day_sent": false,
				"notify_expired_sent": false,
				"updated_at":          now,
			}).Error; err != nil {
				return err
			}
			return tx.Model(&model.OrderItem{}).Where("order_id in (?)", tx.Model(&model.Order{}).Select("id").Where("id = ? or parent_order_id = ?", order.ID, order.ID)).Updates(map[string]interface{}{
				"status":     model.OrderItemStatusActive,
				"updated_at": now,
			}).Error
		}); err != nil {
			return err
		}
		return s.rebuildManagedRuntime(ctx)
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).Where("id = ?", orderID).Updates(map[string]interface{}{
			"status":              model.OrderStatusActive,
			"notify_one_day_sent": false,
			"notify_expired_sent": false,
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.OrderItem{}).Where("order_id = ?", orderID).Updates(map[string]interface{}{
			"status":     model.OrderItemStatusActive,
			"updated_at": now,
		}).Error
	}); err != nil {
		return err
	}

	if order.Mode == model.OrderModeDedicated || order.ParentOrderID != nil {
		return s.rebuildManagedRuntime(ctx)
	}
	return s.SyncOrderRuntime(ctx, orderID)
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

func (s *OrderService) BatchActivate(ctx context.Context, orderIDs []uint) []BatchActionResult {
	out := make([]BatchActionResult, 0, len(orderIDs))
	for _, id := range orderIDs {
		err := s.ActivateOrder(ctx, id)
		entry := BatchActionResult{ID: id, Success: err == nil}
		if err != nil {
			entry.Error = err.Error()
		}
		out = append(out, entry)
	}
	return out
}

func (s *OrderService) RenewOrder(ctx context.Context, orderID uint, moreDays int) error {
	return s.RenewOrderWithExpiresAt(ctx, orderID, moreDays, time.Time{})
}

func (s *OrderService) RenewOrderWithExpiresAt(ctx context.Context, orderID uint, moreDays int, expiresAt time.Time) error {
	if !expiresAt.IsZero() && !expiresAt.After(time.Now()) {
		return errors.New("expires_at must be in the future")
	}
	if moreDays <= 0 {
		moreDays = 30
	}
	var order model.Order
	if err := s.db.First(&order, orderID).Error; err != nil {
		return err
	}
	if order.IsGroupHead {
		return s.renewOrderGroup(ctx, order, moreDays, expiresAt)
	}
	newExpires := expiresAt
	if newExpires.IsZero() {
		base := order.ExpiresAt
		if base.Before(time.Now()) {
			base = time.Now()
		}
		newExpires = base.Add(time.Duration(moreDays) * 24 * time.Hour)
	}
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
	return s.BatchRenewWithExpiresAt(ctx, orderIDs, moreDays, time.Time{})
}

func (s *OrderService) BatchRenewWithExpiresAt(ctx context.Context, orderIDs []uint, moreDays int, expiresAt time.Time) []BatchActionResult {
	out := make([]BatchActionResult, 0, len(orderIDs))
	for _, id := range orderIDs {
		err := s.RenewOrderWithExpiresAt(ctx, id, moreDays, expiresAt)
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

const (
	ResidentialTXTLayoutColon = "colon"
	ResidentialTXTLayoutURI   = "uri"
)

func normalizeResidentialTXTLayout(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ResidentialTXTLayoutURI, "socks5", "socks5_uri", "socks5-url":
		return ResidentialTXTLayoutURI
	default:
		return ResidentialTXTLayoutColon
	}
}

func (s *OrderService) BatchExport(orderIDs []uint, residentialTXTLayout string) (string, string, error) {
	parts := make([]string, 0, len(orderIDs)*4)
	skus := map[string]struct{}{}
	for _, id := range orderIDs {
		lines, filename, err := s.ExportOrderLinesWithMeta(id, ExportOrderOptions{
			Shuffle:              false,
			ResidentialTXTLayout: residentialTXTLayout,
		})
		if err != nil {
			return "", "", err
		}
		parts = append(parts, fmt.Sprintf("# %s", filename))
		parts = append(parts, lines)
	}
	for _, id := range orderIDs {
		order := model.Order{}
		if err := s.db.Select("id", "mode", "dedicated_protocol").First(&order, id).Error; err != nil {
			continue
		}
		skus[exportSKUForOrder(order)] = struct{}{}
	}
	sku := "Batch"
	if len(skus) == 1 {
		for one := range skus {
			sku = one
		}
	}
	return strings.Join(parts, "\n\n"), buildTimestampSKUFilename(time.Now(), sku, "txt"), nil
}

func (s *OrderService) ExportOrderLines(orderID uint) (string, error) {
	lines, _, err := s.ExportOrderLinesWithMeta(orderID, ExportOrderOptions{Shuffle: false})
	return lines, err
}

func (s *OrderService) CopyOrderLinkLines(orderID uint) (string, error) {
	var order model.Order
	if err := s.db.Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").First(&order, orderID).Error; err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
		return "", errors.New("only dedicated order supports copy links")
	}
	ctx, err := s.loadDedicatedExportContext(order)
	if err != nil {
		return "", err
	}
	return strings.Join(ctx.buildLinkLines(false), "\n"), nil
}

func (s *OrderService) ExportOrderLinesWithMeta(orderID uint, opts ExportOrderOptions) (string, string, error) {
	var order model.Order
	if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").First(&order, orderID).Error; err != nil {
		return "", "", err
	}
	ctx, err := s.loadDedicatedExportContext(order)
	if err != nil && !strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
		ctx = dedicatedExportContext{baseOrder: order}
	} else if err != nil {
		return "", "", err
	}
	items := ctx.items

	canShuffle := opts.Shuffle && !strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated)
	if canShuffle {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(items), func(i, j int) {
			items[i], items[j] = items[j], items[i]
		})
	}
	if opts.Count > 0 {
		if opts.Count > len(items) {
			return "", "", fmt.Errorf("extract count %d exceeds active items %d", opts.Count, len(items))
		}
		items = items[:opts.Count]
	}

	lines := make([]string, 0, len(items)+4)
	if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
		rows, err := s.collectXLSXRows([]uint{orderID}, XLSXExportOptions{
			Count:   opts.Count,
			Shuffle: false,
		})
		if err != nil {
			return "", "", err
		}
		return buildDedicatedTXT(rows, exportOrderProtocol(order)), buildTimestampSKUFilename(time.Now(), exportSKUForOrder(order), "txt"), nil
	}
	layout := normalizeResidentialTXTLayout(opts.ResidentialTXTLayout)
	for _, item := range items {
		lines = append(lines, buildResidentialTXTLine(item, layout))
	}
	return strings.Join(lines, "\n"), buildTimestampSKUFilename(time.Now(), exportSKUForOrder(order), "txt"), nil
}

type dedicatedExportContext struct {
	baseOrder       model.Order
	ordersForExport []model.Order
	itemsByOrder    map[uint][]model.OrderItem
	items           []model.OrderItem
	egressByItem    map[uint]model.DedicatedEgress
}

func (s *OrderService) loadDedicatedExportContext(order model.Order) (dedicatedExportContext, error) {
	ctx := dedicatedExportContext{
		baseOrder:       order,
		ordersForExport: []model.Order{order},
		itemsByOrder:    map[uint][]model.OrderItem{},
		egressByItem:    map[uint]model.DedicatedEgress{},
	}
	if order.IsGroupHead {
		children := []model.Order{}
		if err := s.db.Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").Where("parent_order_id = ?", order.ID).Order("sequence_no asc, id asc").Find(&children).Error; err != nil {
			return ctx, err
		}
		if len(children) > 0 {
			ctx.ordersForExport = children
		}
	}
	for _, oneOrder := range ctx.ordersForExport {
		orderItems := []model.OrderItem{}
		if err := s.db.Where("order_id = ? and status = ?", oneOrder.ID, model.OrderItemStatusActive).Order("id asc").Find(&orderItems).Error; err != nil {
			return ctx, err
		}
		ctx.itemsByOrder[oneOrder.ID] = orderItems
		ctx.items = append(ctx.items, orderItems...)
	}
	if len(ctx.items) == 0 {
		return ctx, errors.New("no active items")
	}
	itemIDs := make([]uint, 0, len(ctx.items))
	for _, item := range ctx.items {
		itemIDs = append(itemIDs, item.ID)
	}
	if len(itemIDs) > 0 {
		egressRows := []model.DedicatedEgress{}
		if err := s.db.Where("order_item_id in ?", itemIDs).Find(&egressRows).Error; err == nil {
			for _, row := range egressRows {
				ctx.egressByItem[row.OrderItemID] = row
			}
		}
	}
	return ctx, nil
}

func (ctx dedicatedExportContext) buildLinkLines(includeDomain bool) []string {
	lines := make([]string, 0, len(ctx.items))
	for _, oneOrder := range ctx.ordersForExport {
		for _, item := range ctx.itemsByOrder[oneOrder.ID] {
			egress := ctx.egressByItem[item.ID]
			tag := dedicatedLinkTag(egress.CountryCode, egress.ExitIP)
			parts := make([]string, 0, 5)
			if includeDomain {
				parts = append(parts, dedicatedDomainCredentialLine(oneOrder, item))
			}
			parts = append(parts,
				buildOrderItemLinkByProtocol(oneOrder, item, model.DedicatedFeatureMixed, tag),
				buildOrderItemLinkByProtocol(oneOrder, item, model.DedicatedFeatureVless, tag),
				buildOrderItemLinkByProtocol(oneOrder, item, model.DedicatedFeatureVmess, tag),
				buildOrderItemLinkByProtocol(oneOrder, item, model.DedicatedFeatureShadowsocks, tag),
			)
			lines = append(lines, strings.Join(parts, "; ")+";")
		}
	}
	return lines
}

func (s *OrderService) TestOrder(orderID uint) (map[uint]string, error) {
	return s.TestOrderSampled(orderID, 100)
}

func (s *OrderService) TestOrderSampled(orderID uint, samplePercent int) (map[uint]string, error) {
	order := model.Order{}
	if err := s.db.Select("id", "mode").First(&order, orderID).Error; err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
		return nil, errors.New("dedicated order does not support test")
	}
	var items []model.OrderItem
	if err := s.db.Where("order_id = ? and status = ?", orderID, model.OrderItemStatusActive).Find(&items).Error; err != nil {
		return nil, err
	}
	items = sampleItems(items, samplePercent)
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

func (s *OrderService) TestOrderStream(orderID uint, samplePercent int, emit func(TestOrderStreamEvent) error) error {
	order := model.Order{}
	if err := s.db.Select("id", "mode").First(&order, orderID).Error; err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
		return errors.New("dedicated order does not support stream test")
	}
	var items []model.OrderItem
	if err := s.db.Where("order_id = ? and status = ?", orderID, model.OrderItemStatusActive).Find(&items).Error; err != nil {
		return err
	}
	selected := sampleItems(items, samplePercent)
	if err := emit(TestOrderStreamEvent{Type: "meta", Total: len(items), Sampled: len(selected), SamplePercent: normalizeSamplePercent(samplePercent)}); err != nil {
		return err
	}
	ok := 0
	fail := 0
	for _, item := range selected {
		event := TestOrderStreamEvent{Type: "result", ItemID: item.ID}
		if !item.Managed {
			event.Status = "skip"
			event.Detail = "unmanaged"
			if err := emit(event); err != nil {
				return err
			}
			continue
		}
		socksAddr := fmt.Sprintf("%s:%d", item.IP, item.Port)
		dialer, err := proxy.SOCKS5("tcp", socksAddr, &proxy.Auth{User: item.Username, Password: item.Password}, proxy.Direct)
		if err != nil {
			event.Status = "failed"
			event.Detail = "dialer error"
			fail++
			if err := emit(event); err != nil {
				return err
			}
			continue
		}
		httpTransport := &http.Transport{Dial: dialer.Dial}
		client := &http.Client{Timeout: 6 * time.Second, Transport: httpTransport}
		resp, err := client.Get("https://api.ipify.org")
		if err != nil {
			event.Status = "failed"
			event.Detail = "connect failed"
			fail++
			if err := emit(event); err != nil {
				return err
			}
			continue
		}
		body := make([]byte, 128)
		n, _ := resp.Body.Read(body)
		_ = resp.Body.Close()
		event.Status = "ok"
		event.Detail = strings.TrimSpace(string(body[:n]))
		ok++
		if err := emit(event); err != nil {
			return err
		}
	}
	return emit(TestOrderStreamEvent{Type: "done", SuccessCount: ok, FailureCount: fail})
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
		ip := strings.TrimSpace(parts[0])
		port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || port <= 0 || port > 65535 {
			row.Error = "invalid port"
			rows = append(rows, row)
			continue
		}
		row.IP = ip
		row.Port = port
		row.Username = strings.TrimSpace(parts[2])
		row.Password = strings.TrimSpace(parts[3])
		_, row.IsLocalIP = hostIPs[row.IP]
		if row.IsLocalIP {
			occupied, _ := ProbePort(row.IP, row.Port)
			row.PortOccupied = occupied
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return s.applyImportRowValidation(rows)
}

func (s *OrderService) PreviewImportRows(rows []ImportPreviewRow) ([]ImportPreviewRow, error) {
	if len(rows) == 0 {
		return []ImportPreviewRow{}, nil
	}
	return s.applyImportRowValidation(rows)
}

func (s *OrderService) ImportOrder(ctx context.Context, customerID uint, orderName string, expiresAt time.Time, rows []ImportPreviewRow) (*model.Order, error) {
	if customerID == 0 {
		return nil, errors.New("customer_id required")
	}
	rows, err := s.applyImportRowValidation(rows)
	if err != nil {
		return nil, err
	}
	validRows := make([]ImportPreviewRow, 0, len(rows))
	for _, row := range rows {
		if row.Error == "" {
			validRows = append(validRows, row)
		}
	}
	if len(validRows) == 0 {
		return nil, errors.New("no valid rows, please fix preview errors")
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
		orderNo := buildOrderNo(order.CreatedAt, order.ID)
		if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{"order_no": orderNo, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		order.OrderNo = orderNo
		for _, row := range validRows {
			managed := false
			item := model.OrderItem{
				OrderID:      order.ID,
				IP:           row.IP,
				Port:         row.Port,
				Username:     row.Username,
				Password:     row.Password,
				OutboundType: model.OutboundTypeDirect,
				Managed:      false,
				Status:       model.OrderItemStatusActive,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			if host, ok := hostByIP[row.IP]; ok {
				item.HostIPID = &host.ID
				managed = true
			}
			item.Managed = managed
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

func (s *OrderService) allocateIPs(customerID uint, quantity int, mode string, manualIDs []uint, excludeOrderID uint) ([]model.HostIP, error) {
	if mode == model.OrderModeManual {
		if len(manualIDs) < quantity {
			return nil, errors.New("manual_ip_ids insufficient")
		}
		var rows []model.HostIP
		if err := s.db.Where("id in ? and enabled = 1", manualIDs[:quantity]).Find(&rows).Error; err != nil {
			return nil, err
		}
		rows = filterUsableIPs(rows)
		if len(rows) != quantity {
			return nil, errors.New("some manual ips are invalid, disabled, or not usable public local addresses")
		}
		usedByCustomer, err := s.customerUsedIPSet(customerID, excludeOrderID)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if _, exists := usedByCustomer[row.IP]; exists {
				return nil, fmt.Errorf("ip %s already used by current customer", row.IP)
			}
		}
		return rows, nil
	}

	all, err := s.usableIPPool()
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, errors.New("no enabled public host ips")
	}

	usedByCustomer, err := s.customerUsedIPSet(customerID, excludeOrderID)
	if err != nil {
		return nil, err
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

	if len(candidates) < quantity {
		return nil, fmt.Errorf("available IPs (%d) less than quantity (%d)", len(candidates), quantity)
	}

	buckets := map[int64][]model.HostIP{}
	levels := make([]int64, 0)
	seenLevel := map[int64]struct{}{}
	for _, candidate := range candidates {
		level := usage[candidate.IP]
		buckets[level] = append(buckets[level], candidate)
		if _, ok := seenLevel[level]; !ok {
			seenLevel[level] = struct{}{}
			levels = append(levels, level)
		}
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })

	selected := make([]model.HostIP, 0, quantity)
	seed := int(customerID*131 + excludeOrderID*17 + uint(quantity)*7)
	for _, level := range levels {
		bucket := buckets[level]
		sort.Slice(bucket, func(i, j int) bool {
			return lessIPString(bucket[i].IP, bucket[j].IP)
		})
		need := quantity - len(selected)
		if need <= 0 {
			break
		}
		take := need
		if take > len(bucket) {
			take = len(bucket)
		}
		selected = append(selected, scatteredPick(bucket, take, seed+int(level)*11)...)
	}
	if len(selected) < quantity {
		return nil, fmt.Errorf("available IPs (%d) less than quantity (%d)", len(selected), quantity)
	}
	return selected[:quantity], nil
}

func (s *OrderService) usableIPPool() ([]model.HostIP, error) {
	var all []model.HostIP
	if err := s.db.Where("enabled = 1 and is_local = 1").Find(&all).Error; err != nil {
		return nil, err
	}
	all = filterUsableIPs(all)
	sort.Slice(all, func(i, j int) bool {
		return lessIPString(all[i].IP, all[j].IP)
	})
	return all, nil
}

func (s *OrderService) customerUsedIPSet(customerID uint, excludeOrderID uint) (map[string]struct{}, error) {
	usedByCustomer := map[string]struct{}{}
	if customerID == 0 {
		return usedByCustomer, nil
	}
	var ips []struct{ IP string }
	q := s.db.Table("order_items oi").
		Select("oi.ip").
		Joins("join orders o on o.id = oi.order_id").
		Where("o.customer_id = ? and o.status = ? and o.expires_at > ? and oi.status = ?", customerID, model.OrderStatusActive, time.Now(), model.OrderItemStatusActive)
	if excludeOrderID > 0 {
		q = q.Where("o.id <> ?", excludeOrderID)
	}
	if err := q.Scan(&ips).Error; err != nil {
		return nil, err
	}
	for _, v := range ips {
		usedByCustomer[v.IP] = struct{}{}
	}
	return usedByCustomer, nil
}

func sampleItems(items []model.OrderItem, samplePercent int) []model.OrderItem {
	p := normalizeSamplePercent(samplePercent)
	if len(items) == 0 {
		return items
	}
	if p >= 100 {
		return items
	}
	cloned := append([]model.OrderItem(nil), items...)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(cloned), func(i, j int) {
		cloned[i], cloned[j] = cloned[j], cloned[i]
	})
	n := len(cloned) * p / 100
	if n <= 0 {
		n = 1
	}
	return cloned[:n]
}

func normalizeSamplePercent(p int) int {
	if p == 5 || p == 10 || p == 100 {
		return p
	}
	if p <= 0 {
		return 100
	}
	if p > 100 {
		return 100
	}
	if p < 5 {
		return 5
	}
	return p
}

func buildOrderNo(ts time.Time, id uint) string {
	if ts.IsZero() {
		ts = time.Now()
	}
	return fmt.Sprintf("OD%s%06d", ts.Format("060102"), id)
}

func exportSKUForOrder(order model.Order) string {
	if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
		return exportProtocolLabel(order.DedicatedProtocol)
	}
	return "Home"
}

func buildResidentialTXTLine(item model.OrderItem, layout string) string {
	if normalizeResidentialTXTLayout(layout) == ResidentialTXTLayoutURI {
		return (&url.URL{
			Scheme: "socks5",
			User:   url.UserPassword(strings.TrimSpace(item.Username), strings.TrimSpace(item.Password)),
			Host:   fmt.Sprintf("%s:%d", strings.TrimSpace(item.IP), item.Port),
		}).String()
	}
	return fmt.Sprintf("%s:%d:%s:%s", item.IP, item.Port, item.Username, item.Password)
}

func sanitizeFilenamePart(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, "/", "-")
	v = strings.ReplaceAll(v, "\\", "-")
	v = strings.ReplaceAll(v, ":", "-")
	v = strings.ReplaceAll(v, "|", "-")
	v = strings.ReplaceAll(v, "\"", "")
	v = strings.ReplaceAll(v, "'", "")
	v = strings.ReplaceAll(v, "*", "")
	v = strings.ReplaceAll(v, "?", "")
	v = strings.ReplaceAll(v, "<", "")
	v = strings.ReplaceAll(v, ">", "")
	v = strings.Join(strings.Fields(v), "-")
	return strings.Trim(v, "-. ")
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

func (s *OrderService) residentialNamePrefix() string {
	var row model.Setting
	if err := s.db.First(&row, "key = ?", "residential_name_prefix").Error; err != nil {
		return "家宽-Socks5"
	}
	v := strings.TrimSpace(row.Value)
	if v == "" {
		return "家宽-Socks5"
	}
	return v
}

func normalizeResidentialCredentialMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), ResidentialCredentialModeCustom) {
		return ResidentialCredentialModeCustom
	}
	return ResidentialCredentialModeRandom
}

func normalizeResidentialCredentialStrategy(strategy string) string {
	if strings.EqualFold(strings.TrimSpace(strategy), ResidentialCredentialStrategyShared) {
		return ResidentialCredentialStrategyShared
	}
	return ResidentialCredentialStrategyPerLine
}

func parseResidentialCredentialLines(lines string) ([]ResidentialCredentialLine, error) {
	scanner := bufio.NewScanner(strings.NewReader(lines))
	out := make([]ResidentialCredentialLine, 0)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid credential line %q, expect user:pass", raw)
		}
		row := ResidentialCredentialLine{
			Username: strings.TrimSpace(parts[0]),
			Password: strings.TrimSpace(parts[1]),
		}
		if row.Username == "" || row.Password == "" {
			return nil, fmt.Errorf("invalid credential line %q, username/password required", raw)
		}
		out = append(out, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no valid credential lines")
	}
	return out, nil
}

func expandResidentialCredentialLines(rows []ResidentialCredentialLine, strategy string, quantity int) ([]ResidentialCredentialLine, error) {
	if quantity <= 0 {
		return nil, errors.New("quantity must be > 0")
	}
	if normalizeResidentialCredentialStrategy(strategy) == ResidentialCredentialStrategyShared {
		if len(rows) != 1 {
			return nil, fmt.Errorf("shared credential strategy requires exactly 1 credential line, got %d", len(rows))
		}
		expanded := make([]ResidentialCredentialLine, quantity)
		for i := 0; i < quantity; i++ {
			expanded[i] = rows[0]
		}
		return expanded, nil
	}
	if len(rows) != quantity {
		return nil, fmt.Errorf("credential count %d not equal quantity %d", len(rows), quantity)
	}
	return rows, nil
}

func residentialAssignmentsFromHostIPs(ips []model.HostIP, rows []ResidentialCredentialLine) ([]residentialCredentialAssignment, error) {
	if len(ips) != len(rows) {
		return nil, fmt.Errorf("credential count %d not equal ip count %d", len(rows), len(ips))
	}
	assignments := make([]residentialCredentialAssignment, 0, len(rows))
	for i, ip := range ips {
		assignments = append(assignments, residentialCredentialAssignment{
			IP:       strings.TrimSpace(ip.IP),
			Username: strings.TrimSpace(rows[i].Username),
			Password: rows[i].Password,
		})
	}
	return assignments, nil
}

func residentialAssignmentsFromOrderItems(items []model.OrderItem, rows []ResidentialCredentialLine) ([]residentialCredentialAssignment, error) {
	if len(items) != len(rows) {
		return nil, fmt.Errorf("credential count %d not equal order items %d", len(rows), len(items))
	}
	assignments := make([]residentialCredentialAssignment, 0, len(rows))
	for i, item := range items {
		assignments = append(assignments, residentialCredentialAssignment{
			IP:       strings.TrimSpace(item.IP),
			Username: strings.TrimSpace(rows[i].Username),
			Password: rows[i].Password,
		})
	}
	return assignments, nil
}

func (s *OrderService) ensureResidentialCredentialAssignmentsAvailableTx(tx *gorm.DB, assignments []residentialCredentialAssignment, excludeOrderID uint) error {
	seen := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		ip := strings.TrimSpace(assignment.IP)
		user := strings.TrimSpace(assignment.Username)
		if ip == "" {
			return errors.New("ip required for residential credential validation")
		}
		if user == "" {
			return errors.New("username required")
		}
		key := ip + "\x00" + user
		if _, ok := seen[key]; ok {
			return fmt.Errorf("username %s duplicated for ip %s in credential lines", user, ip)
		}
		seen[key] = struct{}{}
		exists, err := s.residentialCredentialAssignmentExistsTx(tx, ip, user, excludeOrderID)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("username %s already exists for ip %s in database", user, ip)
		}
	}
	return nil
}

func (s *OrderService) nextAvailableResidentialUsernameTx(tx *gorm.DB, ip string, excludeOrderID uint) (string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", errors.New("ip required for residential username generation")
	}
	for i := 0; i < 24; i++ {
		candidate := randomString(8)
		exists, err := s.residentialCredentialAssignmentExistsTx(tx, ip, candidate, excludeOrderID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", errors.New("failed to generate unique residential username")
}

func (s *OrderService) ensurePortReadyForManaged(port int) error {
	var count int64
	if err := s.db.Table("order_items oi").
		Joins("join orders o on o.id = oi.order_id").
		Where("oi.port = ? and oi.managed = 1 and oi.status = ? and o.status = ? and o.expires_at > ?", port, model.OrderItemStatusActive, model.OrderStatusActive, time.Now()).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	occupied, probeErr := ProbePort("0.0.0.0", port)
	if probeErr != nil {
		return probeErr
	}
	if occupied {
		return fmt.Errorf("target port %d already occupied", port)
	}
	return nil
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

func (s *OrderService) applyImportRowValidation(rows []ImportPreviewRow) ([]ImportPreviewRow, error) {
	checked := make([]ImportPreviewRow, len(rows))
	copy(checked, rows)

	for i := range checked {
		checked[i].IP = strings.TrimSpace(checked[i].IP)
		checked[i].Username = strings.TrimSpace(checked[i].Username)
		checked[i].Password = strings.TrimSpace(checked[i].Password)
		if checked[i].Error != "" {
			continue
		}
		if checked[i].IP == "" {
			checked[i].Error = "invalid ip"
			continue
		}
		if checked[i].Port <= 0 || checked[i].Port > 65535 {
			checked[i].Error = "invalid port"
			continue
		}
		if checked[i].Username == "" {
			checked[i].Error = "username required"
			continue
		}
		if checked[i].Password == "" {
			checked[i].Error = "password required"
			continue
		}
	}

	return checked, nil
}

func uniqueUintIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return []uint{}
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *OrderService) loadForwardOutboundsByIDs(ids []uint, enabledOnly bool) ([]model.SocksOutbound, error) {
	return s.loadForwardOutboundsByIDsTx(s.db, ids, enabledOnly)
}

func (s *OrderService) loadForwardOutboundsByIDsTx(tx *gorm.DB, ids []uint, enabledOnly bool) ([]model.SocksOutbound, error) {
	ids = uniqueUintIDs(ids)
	if len(ids) == 0 {
		return []model.SocksOutbound{}, nil
	}
	rows := []model.SocksOutbound{}
	q := tx.Where("id in ?", ids)
	if enabledOnly {
		q = q.Where("enabled = 1")
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]model.SocksOutbound, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ordered := make([]model.SocksOutbound, 0, len(ids))
	missing := make([]string, 0)
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			missing = append(missing, strconv.FormatUint(uint64(id), 10))
			continue
		}
		ordered = append(ordered, row)
	}
	if len(missing) > 0 {
		if enabledOnly {
			return nil, fmt.Errorf("some forward outbounds not found or disabled: %s", strings.Join(missing, ","))
		}
		return nil, fmt.Errorf("some forward outbounds not found: %s", strings.Join(missing, ","))
	}
	return ordered, nil
}

func (s *OrderService) syncForwardOrderItemsTx(tx *gorm.DB, order model.Order, targetOutboundIDs []uint, targetPort int, targetExpiresAt time.Time, now time.Time) error {
	targetOutboundIDs = uniqueUintIDs(targetOutboundIDs)
	outbounds, err := s.loadForwardOutboundsByIDsTx(tx, targetOutboundIDs, true)
	if err != nil {
		return err
	}
	outboundByID := make(map[uint]model.SocksOutbound, len(outbounds))
	targetSet := make(map[uint]struct{}, len(outbounds))
	for _, outbound := range outbounds {
		outboundByID[outbound.ID] = outbound
		targetSet[outbound.ID] = struct{}{}
	}

	keepByOutbound := map[uint]model.OrderItem{}
	removeIDs := make([]uint, 0)
	for _, item := range order.Items {
		if item.SocksOutboundID == nil || *item.SocksOutboundID == 0 || !strings.EqualFold(strings.TrimSpace(item.OutboundType), model.OutboundTypeSocks5) {
			removeIDs = append(removeIDs, item.ID)
			continue
		}
		outboundID := *item.SocksOutboundID
		if _, ok := targetSet[outboundID]; !ok {
			removeIDs = append(removeIDs, item.ID)
			continue
		}
		if prev, exists := keepByOutbound[outboundID]; exists {
			if item.ID > prev.ID {
				removeIDs = append(removeIDs, item.ID)
				continue
			}
			removeIDs = append(removeIDs, prev.ID)
		}
		keepByOutbound[outboundID] = item
	}

	if len(removeIDs) > 0 {
		if err := tx.Where("order_item_id in ?", removeIDs).Delete(&model.XrayResource{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", removeIDs).Delete(&model.OrderItem{}).Error; err != nil {
			return err
		}
	}

	addOutboundIDs := make([]uint, 0)
	for _, outboundID := range targetOutboundIDs {
		if _, ok := keepByOutbound[outboundID]; ok {
			continue
		}
		addOutboundIDs = append(addOutboundIDs, outboundID)
	}

	if len(addOutboundIDs) > 0 {
		ips, err := s.allocateIPs(order.CustomerID, len(addOutboundIDs), model.OrderModeAuto, nil, order.ID)
		if err != nil {
			return err
		}
		for i, outboundID := range addOutboundIDs {
			outbound := outboundByID[outboundID]
			user, userErr := s.resolveForwardItemUsername(tx, outbound, now, 0)
			if userErr != nil {
				return userErr
			}
			item := model.OrderItem{
				OrderID:         order.ID,
				HostIPID:        &ips[i].ID,
				IP:              ips[i].IP,
				Port:            targetPort,
				Username:        user,
				Password:        randomString(12),
				SocksOutboundID: &outbound.ID,
				OutboundType:    model.OutboundTypeSocks5,
				ForwardAddress:  outbound.Address,
				ForwardPort:     outbound.Port,
				ForwardUsername: outbound.Username,
				ForwardPassword: outbound.Password,
				Managed:         true,
				Status:          model.OrderItemStatusActive,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
	}

	current := []model.OrderItem{}
	if err := tx.Where("order_id = ?", order.ID).Find(&current).Error; err != nil {
		return err
	}
	for _, item := range current {
		if item.SocksOutboundID == nil || *item.SocksOutboundID == 0 {
			continue
		}
		outbound, ok := outboundByID[*item.SocksOutboundID]
		if !ok {
			continue
		}
		user, userErr := s.resolveForwardItemUsername(tx, outbound, now, item.ID)
		if userErr != nil {
			return userErr
		}
		updates := map[string]interface{}{
			"port":              targetPort,
			"username":          user,
			"socks_outbound_id": outbound.ID,
			"outbound_type":     model.OutboundTypeSocks5,
			"forward_address":   outbound.Address,
			"forward_port":      outbound.Port,
			"forward_username":  outbound.Username,
			"forward_password":  outbound.Password,
			"managed":           true,
			"updated_at":        now,
		}
		if targetExpiresAt.After(now) {
			updates["status"] = model.OrderItemStatusActive
		}
		if err := tx.Model(&model.OrderItem{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *OrderService) ForwardOutboundReuseWarnings(customerID uint, excludeOrderID uint, outboundIDs []uint) ([]string, error) {
	outboundIDs = uniqueUintIDs(outboundIDs)
	if customerID == 0 || len(outboundIDs) == 0 {
		return []string{}, nil
	}
	type warnRow struct {
		OutboundID uint
		Address    string
		Port       int
		RouteUser  string
		Count      int64
	}
	rows := []warnRow{}
	q := s.db.Table("order_items oi").
		Select("oi.socks_outbound_id as outbound_id, so.address as address, so.port as port, so.route_user as route_user, count(1) as count").
		Joins("join orders o on o.id = oi.order_id").
		Joins("join socks_outbounds so on so.id = oi.socks_outbound_id").
		Where("o.customer_id = ? and o.status = ? and o.expires_at > ? and oi.status = ? and oi.socks_outbound_id in ?", customerID, model.OrderStatusActive, time.Now(), model.OrderItemStatusActive, outboundIDs)
	if excludeOrderID > 0 {
		q = q.Where("o.id <> ?", excludeOrderID)
	}
	if err := q.Group("oi.socks_outbound_id, so.address, so.port, so.route_user").Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []string{}, nil
	}
	warnings := make([]string, 0, len(rows))
	for _, row := range rows {
		user := strings.TrimSpace(row.RouteUser)
		if user == "" {
			user = "(未设置分流用户)"
		}
		warnings = append(warnings, fmt.Sprintf("SOCKS5出口 %s:%d (%s) 已在该客户其他活动订单复用 %d 次", row.Address, row.Port, user, row.Count))
	}
	sort.Strings(warnings)
	return warnings, nil
}

func (s *OrderService) allocateForwardOutbounds(customerID uint, quantity int, excludeOrderID uint) ([]model.SocksOutbound, error) {
	all := []model.SocksOutbound{}
	if err := s.db.Where("enabled = 1").Order("id asc").Find(&all).Error; err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, errors.New("no enabled socks outbounds")
	}

	usedByCustomer, err := s.customerUsedOutboundSet(customerID, excludeOrderID)
	if err != nil {
		return nil, err
	}

	usage := map[uint]int64{}
	usageRows := []struct {
		OutboundID uint
		Count      int64
	}{}
	if err := s.db.Table("order_items oi").
		Select("oi.socks_outbound_id as outbound_id, count(1) as count").
		Joins("join orders o on o.id = oi.order_id").
		Where("oi.socks_outbound_id is not null and oi.status = ? and o.status = ? and o.expires_at > ?", model.OrderItemStatusActive, model.OrderStatusActive, time.Now()).
		Group("oi.socks_outbound_id").Scan(&usageRows).Error; err != nil {
		return nil, err
	}
	for _, row := range usageRows {
		usage[row.OutboundID] = row.Count
	}

	preferred := make([]model.SocksOutbound, 0, len(all))
	fallback := make([]model.SocksOutbound, 0, len(all))
	for _, outbound := range all {
		if _, exists := usedByCustomer[outbound.ID]; exists {
			fallback = append(fallback, outbound)
			continue
		}
		preferred = append(preferred, outbound)
	}

	selected := make([]model.SocksOutbound, 0, quantity)
	seed := int(customerID*163 + excludeOrderID*13 + uint(quantity)*19)
	selected = append(selected, selectDispersedOutbounds(preferred, usage, quantity, seed)...)
	if len(selected) < quantity {
		need := quantity - len(selected)
		selected = append(selected, selectDispersedOutbounds(fallback, usage, need, seed+97)...)
	}
	if len(selected) < quantity {
		return nil, fmt.Errorf("available outbounds (%d) less than quantity (%d)", len(selected), quantity)
	}
	return selected[:quantity], nil
}

func selectDispersedOutbounds(candidates []model.SocksOutbound, usage map[uint]int64, quantity int, seed int) []model.SocksOutbound {
	if quantity <= 0 || len(candidates) == 0 {
		return []model.SocksOutbound{}
	}
	buckets := map[int64][]model.SocksOutbound{}
	levels := make([]int64, 0)
	seen := map[int64]struct{}{}
	for _, outbound := range candidates {
		level := usage[outbound.ID]
		buckets[level] = append(buckets[level], outbound)
		if _, ok := seen[level]; !ok {
			seen[level] = struct{}{}
			levels = append(levels, level)
		}
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	out := make([]model.SocksOutbound, 0, quantity)
	for _, level := range levels {
		bucket := buckets[level]
		sort.Slice(bucket, func(i, j int) bool {
			li := strings.TrimSpace(bucket[i].RouteUser)
			lj := strings.TrimSpace(bucket[j].RouteUser)
			if li == lj {
				return bucket[i].ID < bucket[j].ID
			}
			return li < lj
		})
		need := quantity - len(out)
		if need <= 0 {
			break
		}
		take := need
		if take > len(bucket) {
			take = len(bucket)
		}
		out = append(out, scatteredPick(bucket, take, seed+int(level)*7)...)
	}
	return out
}

func (s *OrderService) customerUsedOutboundSet(customerID uint, excludeOrderID uint) (map[uint]struct{}, error) {
	used := map[uint]struct{}{}
	if customerID == 0 {
		return used, nil
	}
	rows := []struct {
		OutboundID uint
	}{}
	q := s.db.Table("order_items oi").
		Select("oi.socks_outbound_id as outbound_id").
		Joins("join orders o on o.id = oi.order_id").
		Where("o.customer_id = ? and o.status = ? and o.expires_at > ? and oi.status = ? and oi.socks_outbound_id is not null", customerID, model.OrderStatusActive, time.Now(), model.OrderItemStatusActive)
	if excludeOrderID > 0 {
		q = q.Where("o.id <> ?", excludeOrderID)
	}
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.OutboundID > 0 {
			used[row.OutboundID] = struct{}{}
		}
	}
	return used, nil
}

func (s *OrderService) resolveForwardItemUsername(tx *gorm.DB, outbound model.SocksOutbound, now time.Time, excludeItemID uint) (string, error) {
	user := strings.TrimSpace(outbound.RouteUser)
	if user != "" {
		exists, err := s.orderUsernameExistsTx(tx, user, excludeItemID)
		if err != nil {
			return "", err
		}
		if !exists {
			return user, nil
		}
	}

	generated, err := s.nextAvailableOrderUsernameTx(tx, outbound.CountryCode)
	if err != nil {
		return "", err
	}
	if err := tx.Model(&model.SocksOutbound{}).Where("id = ?", outbound.ID).Updates(map[string]interface{}{
		"route_user": generated,
		"updated_at": now,
	}).Error; err != nil {
		return "", err
	}
	return generated, nil
}

func (s *OrderService) nextAvailableOrderUsername(countryCode string) (string, error) {
	return s.nextAvailableOrderUsernameTx(s.db, countryCode)
}

func (s *OrderService) nextAvailableOrderUsernameTx(tx *gorm.DB, countryCode string) (string, error) {
	prefix := normalizeCountryPrefix(countryCode)
	for i := 0; i < 16; i++ {
		candidate := fmt.Sprintf("%s-%s", prefix, randomString(10))
		exists, err := s.orderUsernameExistsTx(tx, candidate, 0)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", errors.New("failed to generate unique order username")
}

func (s *OrderService) orderUsernameExistsTx(tx *gorm.DB, username string, excludeItemID uint) (bool, error) {
	var count int64
	q := tx.Model(&model.OrderItem{}).Where("username = ?", strings.TrimSpace(username))
	if excludeItemID > 0 {
		q = q.Where("id <> ?", excludeItemID)
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *OrderService) residentialCredentialAssignmentExistsTx(tx *gorm.DB, ip string, username string, excludeOrderID uint) (bool, error) {
	var count int64
	q := tx.Model(&model.OrderItem{}).
		Where("ip = ? and username = ?", strings.TrimSpace(ip), strings.TrimSpace(username))
	if excludeOrderID > 0 {
		q = q.Where("order_id <> ?", excludeOrderID)
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func scatteredPick[T any](rows []T, take int, seed int) []T {
	n := len(rows)
	if take <= 0 || n == 0 {
		return []T{}
	}
	if take >= n {
		out := make([]T, n)
		copy(out, rows)
		return out
	}
	if seed < 0 {
		seed = -seed
	}
	start := seed % n
	stride := n / take
	if stride <= 1 {
		stride = 2
	}
	for gcd(stride, n) != 1 {
		stride++
	}
	used := make([]bool, n)
	out := make([]T, 0, take)
	idx := start
	for len(out) < take {
		if !used[idx] {
			used[idx] = true
			out = append(out, rows[idx])
		}
		idx = (idx + stride) % n
	}
	return out
}

func gcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists")
}

func filterUsableIPs(rows []model.HostIP) []model.HostIP {
	out := make([]model.HostIP, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled || !row.IsLocal || !row.IsPublic {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(row.IP))
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			continue
		}
		out = append(out, row)
	}
	return out
}
