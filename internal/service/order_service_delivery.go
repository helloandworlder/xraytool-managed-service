package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const dedicatedEgressProbeParallelism = 16

var (
	dedicatedEgressGeoProbe      = probeSocksOutboundGeo
	dedicatedEgressAsyncDispatch = func(fn func()) { go fn() }
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

type dedicatedEgressProbeTarget struct {
	Index    int
	Raw      string
	EgressID uint
	OrderID  uint
	Address  string
	Port     int
	Username string
	Password string
}

type dedicatedEgressProbeResult struct {
	Target      dedicatedEgressProbeTarget
	ProbedAt    time.Time
	Available   bool
	ExitIP      string
	CountryCode string
	Region      string
	Error       string
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
	targets := make([]dedicatedEgressProbeTarget, 0, len(rows))
	for i, row := range rows {
		targets = append(targets, dedicatedEgressProbeTarget{
			Index:    i + 1,
			Raw:      fmt.Sprintf("%s:%d:%s:%s", row.Address, row.Port, row.Username, row.Password),
			Address:  row.Address,
			Port:     row.Port,
			Username: row.Username,
			Password: row.Password,
		})
	}
	success := 0
	failed := 0
	if err := runDedicatedEgressProbeTargets(targets, func(result dedicatedEgressProbeResult) error {
		event := DedicatedEgressProbeEvent{
			Type:     "result",
			Index:    result.Target.Index,
			Raw:      result.Target.Raw,
			Address:  result.Target.Address,
			Port:     result.Target.Port,
			Username: result.Target.Username,
			Password: result.Target.Password,
		}
		if !result.Available {
			failed++
			event.Available = false
			event.Error = result.Error
		} else {
			success++
			event.Available = true
			event.ExitIP = strings.TrimSpace(result.ExitIP)
			event.CountryCode = strings.ToLower(strings.TrimSpace(result.CountryCode))
			event.Region = strings.TrimSpace(result.Region)
		}
		return emit(event)
	}); err != nil {
		return err
	}
	return emit(DedicatedEgressProbeEvent{Type: "done", Total: len(rows), Success: success, Failed: failed})
}

func markDedicatedEgressProbePending(row *model.DedicatedEgress) {
	if row == nil {
		return
	}
	row.ExitIP = ""
	row.CountryCode = ""
	row.Region = ""
	row.ProbeStatus = "pending"
	row.ProbeError = ""
	row.LastProbedAt = nil
}

func newDedicatedEgressProbeTarget(index int, egressID uint, orderID uint, address string, port int, username string, password string) dedicatedEgressProbeTarget {
	return dedicatedEgressProbeTarget{
		Index:    index,
		Raw:      fmt.Sprintf("%s:%d:%s:%s", address, port, username, password),
		EgressID: egressID,
		OrderID:  orderID,
		Address:  address,
		Port:     port,
		Username: username,
		Password: password,
	}
}

func probeDedicatedEgressTarget(target dedicatedEgressProbeTarget) dedicatedEgressProbeResult {
	result := dedicatedEgressProbeResult{
		Target:   target,
		ProbedAt: time.Now(),
	}
	exitIP, country, region, err := dedicatedEgressGeoProbe(target.Address, target.Port, target.Username, target.Password)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Available = true
	result.ExitIP = strings.TrimSpace(exitIP)
	result.CountryCode = strings.ToLower(strings.TrimSpace(country))
	result.Region = strings.TrimSpace(region)
	return result
}

func runDedicatedEgressProbeTargets(targets []dedicatedEgressProbeTarget, handle func(dedicatedEgressProbeResult) error) error {
	if len(targets) == 0 {
		return nil
	}
	workers := dedicatedEgressProbeParallelism
	if workers > len(targets) {
		workers = len(targets)
	}

	jobs := make(chan dedicatedEgressProbeTarget)
	results := make(chan dedicatedEgressProbeResult, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				results <- probeDedicatedEgressTarget(target)
			}
		}()
	}

	go func() {
		for _, target := range targets {
			jobs <- target
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for result := range results {
		if handle == nil {
			continue
		}
		if err := handle(result); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *OrderService) scheduleDedicatedEgressRefresh(targets []dedicatedEgressProbeTarget) {
	if len(targets) == 0 {
		return
	}
	copied := append([]dedicatedEgressProbeTarget(nil), targets...)
	dedicatedEgressAsyncDispatch(func() {
		if err := s.refreshDedicatedEgressTargets(copied); err != nil && s.log != nil {
			s.log.Warn("refresh dedicated egresses async failed", zap.Error(err), zap.Int("count", len(copied)))
		}
	})
}

func (s *OrderService) refreshDedicatedEgressTargets(targets []dedicatedEgressProbeTarget) error {
	var firstErr error
	_ = runDedicatedEgressProbeTargets(targets, func(result dedicatedEgressProbeResult) error {
		if result.Target.EgressID == 0 {
			return nil
		}
		updates := map[string]interface{}{
			"last_probed_at": &result.ProbedAt,
			"updated_at":     result.ProbedAt,
		}
		if result.Available {
			updates["exit_ip"] = result.ExitIP
			updates["country_code"] = result.CountryCode
			updates["region"] = result.Region
			updates["probe_status"] = "ok"
			updates["probe_error"] = ""
		} else {
			updates["exit_ip"] = ""
			updates["country_code"] = ""
			updates["region"] = ""
			updates["probe_status"] = "failed"
			updates["probe_error"] = result.Error
		}
		if err := s.db.Model(&model.DedicatedEgress{}).Where("id = ?", result.Target.EgressID).Updates(updates).Error; err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if s.log != nil {
				s.log.Warn(
					"update dedicated egress probe result failed",
					zap.Error(err),
					zap.Uint("egress_id", result.Target.EgressID),
					zap.Uint("order_id", result.Target.OrderID),
				)
			}
		}
		return nil
	})
	return firstErr
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
		return s.refreshResidentialCredentialsTx(tx, order, now)
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) refreshResidentialCredentialsTx(tx *gorm.DB, order model.Order, now time.Time) error {
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
		username, err := s.nextAvailableResidentialUsernameTx(tx, item.IP, order.ID)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.OrderItem{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
			"username":   username,
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
