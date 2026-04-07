package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
)

type ResidentialUsernameConflictMember struct {
	OrderID      uint      `json:"order_id"`
	OrderNo      string    `json:"order_no"`
	OrderName    string    `json:"order_name"`
	CustomerID   uint      `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	CustomerCode string    `json:"customer_code"`
	Mode         string    `json:"mode"`
	IP           string    `json:"ip"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type ResidentialUsernameConflict struct {
	Username         string                              `json:"username"`
	OrderCount       int                                 `json:"order_count"`
	ItemCount        int                                 `json:"item_count"`
	AffectedOrderIDs []uint                              `json:"affected_order_ids"`
	Members          []ResidentialUsernameConflictMember `json:"members"`
}

func (s *OrderService) ListResidentialUsernameConflicts() ([]ResidentialUsernameConflict, error) {
	type row struct {
		OrderID      uint
		OrderNo      string
		OrderName    string
		CustomerID   uint
		CustomerName string
		CustomerCode string
		Mode         string
		IP           string
		Username     string
		ExpiresAt    time.Time
	}

	rows := make([]row, 0)
	if err := s.db.Table("order_items oi").
		Select(`
			o.id as order_id,
			o.order_no as order_no,
			o.name as order_name,
			o.customer_id as customer_id,
			c.name as customer_name,
			c.code as customer_code,
			o.mode as mode,
			oi.ip as ip,
			oi.username as username,
			o.expires_at as expires_at
		`).
		Joins("join orders o on o.id = oi.order_id").
		Joins("join customers c on c.id = o.customer_id").
		Where("o.status = ? and o.expires_at > ? and oi.status = ?", model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
		Where("o.mode in ?", []string{model.OrderModeAuto, model.OrderModeManual, model.OrderModeImport}).
		Where("oi.username <> ''").
		Order("oi.username asc, o.id asc, oi.id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	conflictsByUsername := map[string]*ResidentialUsernameConflict{}
	orderSets := map[string]map[uint]struct{}{}
	for _, item := range rows {
		username := strings.TrimSpace(item.Username)
		if username == "" {
			continue
		}
		if _, ok := conflictsByUsername[username]; !ok {
			conflictsByUsername[username] = &ResidentialUsernameConflict{
				Username:         username,
				AffectedOrderIDs: []uint{},
				Members:          []ResidentialUsernameConflictMember{},
			}
			orderSets[username] = map[uint]struct{}{}
		}
		conflict := conflictsByUsername[username]
		conflict.ItemCount++
		conflict.Members = append(conflict.Members, ResidentialUsernameConflictMember{
			OrderID:      item.OrderID,
			OrderNo:      item.OrderNo,
			OrderName:    item.OrderName,
			CustomerID:   item.CustomerID,
			CustomerName: item.CustomerName,
			CustomerCode: item.CustomerCode,
			Mode:         strings.ToLower(strings.TrimSpace(item.Mode)),
			IP:           strings.TrimSpace(item.IP),
			ExpiresAt:    item.ExpiresAt,
		})
		if _, exists := orderSets[username][item.OrderID]; !exists {
			orderSets[username][item.OrderID] = struct{}{}
			conflict.AffectedOrderIDs = append(conflict.AffectedOrderIDs, item.OrderID)
		}
	}

	result := make([]ResidentialUsernameConflict, 0)
	for username, conflict := range conflictsByUsername {
		conflict.OrderCount = len(orderSets[username])
		if conflict.OrderCount <= 1 {
			continue
		}
		sort.Slice(conflict.AffectedOrderIDs, func(i, j int) bool {
			return conflict.AffectedOrderIDs[i] < conflict.AffectedOrderIDs[j]
		})
		result = append(result, *conflict)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].OrderCount != result[j].OrderCount {
			return result[i].OrderCount > result[j].OrderCount
		}
		if result[i].ItemCount != result[j].ItemCount {
			return result[i].ItemCount > result[j].ItemCount
		}
		return result[i].Username < result[j].Username
	})
	return result, nil
}

func (s *OrderService) RepairResidentialUsernameConflicts(ctx context.Context, orderIDs []uint) []BatchActionResult {
	targets := uniqueUintIDs(orderIDs)
	results := make([]BatchActionResult, 0, len(targets))
	if len(targets) == 0 {
		return results
	}

	for _, orderID := range targets {
		results = append(results, BatchActionResult{ID: orderID, Success: false})
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for i, orderID := range targets {
			order := model.Order{}
			if err := tx.First(&order, orderID).Error; err != nil {
				results[i].Error = err.Error()
				continue
			}
			if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
				results[i].Error = "dedicated order does not support residential credential refresh"
				continue
			}
			if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeForward) {
				results[i].Error = "forward mode is deprecated"
				continue
			}
			if err := s.refreshResidentialCredentialsTx(tx, order, now); err != nil {
				results[i].Error = err.Error()
				continue
			}
			results[i].Success = true
		}
		return nil
	})
	if err != nil {
		for i := range results {
			if results[i].Success || results[i].Error != "" {
				continue
			}
			results[i].Error = err.Error()
		}
		return results
	}

	if hasBatchSuccess(results) {
		if rebuildErr := s.rebuildManagedRuntime(ctx); rebuildErr != nil {
			for i := range results {
				if !results[i].Success {
					continue
				}
				results[i].Success = false
				results[i].Error = rebuildErr.Error()
			}
		}
	}
	return results
}

func hasBatchSuccess(results []BatchActionResult) bool {
	for _, row := range results {
		if row.Success {
			return true
		}
	}
	return false
}
