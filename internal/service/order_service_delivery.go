package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
)

type DedicatedEgressProbeEvent struct {
	Type        string `json:"type"`
	Index       int    `json:"index,omitempty"`
	Raw         string `json:"raw,omitempty"`
	Address     string `json:"address,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Available   bool   `json:"available,omitempty"`
	ExitIP      string `json:"exit_ip,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	Region      string `json:"region,omitempty"`
	Error       string `json:"error,omitempty"`
	Total       int    `json:"total,omitempty"`
	Success     int    `json:"success,omitempty"`
	Failed      int    `json:"failed,omitempty"`
}

func (s *OrderService) ProbeDedicatedEgressStream(lines string, emit func(DedicatedEgressProbeEvent) error) error {
	rows, err := parseDedicatedEgressLines(lines)
	if err != nil {
		return err
	}
	if emit == nil {
		return errors.New("emit callback is required")
	}
	if err := emit(DedicatedEgressProbeEvent{Type: "start", Total: len(rows)}); err != nil {
		return err
	}
	success := 0
	failed := 0
	for i, row := range rows {
		event := DedicatedEgressProbeEvent{
			Type:     "result",
			Index:    i + 1,
			Raw:      fmt.Sprintf("%s:%d:%s:%s", row.Address, row.Port, row.Username, row.Password),
			Address:  row.Address,
			Port:     row.Port,
			Username: row.Username,
			Password: row.Password,
		}
		exitIP, country, region, probeErr := probeSocksOutboundGeo(row.Address, row.Port, row.Username, row.Password)
		if probeErr != nil {
			failed++
			event.Available = false
			event.Error = probeErr.Error()
		} else {
			success++
			event.Available = true
			event.ExitIP = strings.TrimSpace(exitIP)
			event.CountryCode = strings.ToLower(strings.TrimSpace(country))
			event.Region = strings.TrimSpace(region)
		}
		if err := emit(event); err != nil {
			return err
		}
	}
	return emit(DedicatedEgressProbeEvent{Type: "done", Total: len(rows), Success: success, Failed: failed})
}

func (s *OrderService) RefreshResidentialCredentials(ctx context.Context, orderID uint) error {
	order := model.Order{}
	if err := s.db.First(&order, orderID).Error; err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeDedicated) {
		return errors.New("dedicated order does not support residential credential refresh")
	}
	if strings.EqualFold(strings.TrimSpace(order.Mode), model.OrderModeForward) {
		return errors.New("forward mode is deprecated")
	}
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		targetOrderIDs := []uint{order.ID}
		if order.IsGroupHead {
			childIDs := []uint{}
			if err := tx.Model(&model.Order{}).Where("parent_order_id = ?", order.ID).Pluck("id", &childIDs).Error; err != nil {
				return err
			}
			if len(childIDs) > 0 {
				targetOrderIDs = childIDs
			}
		}
		items := []model.OrderItem{}
		if err := tx.Where("order_id in ?", targetOrderIDs).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			if err := tx.Model(&model.OrderItem{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
				"username":   randomString(8),
				"password":   randomString(12),
				"vmess_uuid": "",
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.Order{}).Where("id in ? or parent_order_id in ?", targetOrderIDs, targetOrderIDs).Updates(map[string]interface{}{
			"updated_at": now,
		}).Error
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) DeleteOrder(ctx context.Context, orderID uint) error {
	order := model.Order{}
	if err := s.db.First(&order, orderID).Error; err != nil {
		return err
	}
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if order.IsGroupHead {
			ids := []uint{}
			if err := tx.Model(&model.Order{}).Where("id = ? or parent_order_id = ?", order.ID, order.ID).Pluck("id", &ids).Error; err != nil {
				return err
			}
			return deleteOrdersByIDsTx(tx, ids)
		}

		if order.ParentOrderID != nil && *order.ParentOrderID > 0 {
			parentID := *order.ParentOrderID
			if err := deleteOrdersByIDsTx(tx, []uint{order.ID}); err != nil {
				return err
			}
			children := []model.Order{}
			if err := tx.Select("id", "status", "expires_at").Where("parent_order_id = ?", parentID).Find(&children).Error; err != nil {
				return err
			}
			if len(children) == 0 {
				return deleteOrdersByIDsTx(tx, []uint{parentID})
			}
			maxExpires := children[0].ExpiresAt
			status := model.OrderStatusExpired
			for _, child := range children {
				if child.ExpiresAt.After(maxExpires) {
					maxExpires = child.ExpiresAt
				}
				if child.Status == model.OrderStatusActive && child.ExpiresAt.After(now) {
					status = model.OrderStatusActive
				}
				if status != model.OrderStatusActive && child.Status == model.OrderStatusDisabled {
					status = model.OrderStatusDisabled
				}
			}
			return tx.Model(&model.Order{}).Where("id = ?", parentID).Updates(map[string]interface{}{
				"quantity":            len(children),
				"status":              status,
				"expires_at":          maxExpires,
				"notify_one_day_sent": false,
				"notify_expired_sent": false,
				"updated_at":          now,
			}).Error
		}

		return deleteOrdersByIDsTx(tx, []uint{order.ID})
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func deleteOrdersByIDsTx(tx *gorm.DB, orderIDs []uint) error {
	ids := uniqueUintIDs(orderIDs)
	if len(ids) == 0 {
		return nil
	}
	itemIDs := []uint{}
	if err := tx.Model(&model.OrderItem{}).Where("order_id in ?", ids).Pluck("id", &itemIDs).Error; err != nil {
		return err
	}
	if len(itemIDs) > 0 {
		if err := tx.Where("order_item_id in ?", itemIDs).Delete(&model.XrayResource{}).Error; err != nil {
			return err
		}
		if err := tx.Where("order_item_id in ?", itemIDs).Delete(&model.DedicatedEgress{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("order_id in ?", ids).Delete(&model.OrderItem{}).Error; err != nil {
		return err
	}
	return tx.Where("id in ?", ids).Delete(&model.Order{}).Error
}
