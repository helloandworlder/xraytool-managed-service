package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *OrderService) createDedicatedOrder(ctx context.Context, in CreateOrderInput, now time.Time, expiresAt time.Time) (*model.Order, error) {
	if in.DedicatedEntryID == 0 {
		return nil, errors.New("dedicated_entry_id is required for dedicated mode")
	}
	entry := model.DedicatedEntry{}
	if err := s.db.Where("id = ? and enabled = 1", in.DedicatedEntryID).First(&entry).Error; err != nil {
		return nil, fmt.Errorf("dedicated entry invalid: %w", err)
	}
	primaryPort := chooseDedicatedPrimaryPort(entry)
	if primaryPort <= 0 {
		return nil, errors.New("dedicated entry has no usable protocol port")
	}
	egressRows, err := parseDedicatedEgressLines(in.DedicatedEgressLines)
	if err != nil {
		return nil, err
	}
	if len(egressRows) == 0 {
		return nil, errors.New("no valid socks5 lines")
	}
	in.Quantity = len(egressRows)
	baseName := strings.TrimSpace(in.Name)
	if baseName == "" {
		baseName = fmt.Sprintf("Dedicated-%d-%s", in.CustomerID, now.Format("20060102150405"))
	}

	head := &model.Order{
		CustomerID:       in.CustomerID,
		Name:             baseName,
		Mode:             model.OrderModeDedicated,
		Status:           model.OrderStatusActive,
		Quantity:         len(egressRows),
		Port:             primaryPort,
		StartsAt:         now,
		ExpiresAt:        expiresAt,
		IsGroupHead:      true,
		DedicatedEntryID: &entry.ID,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(head).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).Where("id = ?", head.ID).Updates(map[string]interface{}{
			"group_id":   head.ID,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		for i, outbound := range egressRows {
			childName := fmt.Sprintf("%s-%03d", baseName, i+1)
			seq := i + 1
			parentID := head.ID
			child := model.Order{
				CustomerID:       in.CustomerID,
				GroupID:          head.ID,
				ParentOrderID:    &parentID,
				SequenceNo:       seq,
				DedicatedEntryID: &entry.ID,
				Name:             childName,
				Mode:             model.OrderModeDedicated,
				Status:           model.OrderStatusActive,
				Quantity:         1,
				Port:             primaryPort,
				StartsAt:         now,
				ExpiresAt:        expiresAt,
			}
			if err := tx.Create(&child).Error; err != nil {
				return err
			}
			item := model.OrderItem{
				OrderID:         child.ID,
				IP:              "127.0.0.1",
				Port:            primaryPort,
				Username:        randomString(8),
				Password:        randomString(12),
				VmessUUID:       randomUUID(),
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
			egress := model.DedicatedEgress{
				OrderID:     child.ID,
				OrderItemID: item.ID,
				Address:     outbound.Address,
				Port:        outbound.Port,
				Username:    outbound.Username,
				Password:    outbound.Password,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Create(&egress).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.rebuildManagedRuntime(ctx); err != nil {
		s.log.Warn("sync runtime after dedicated create failed", zap.Error(err), zap.Uint("order_id", head.ID))
	}

	if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("Items").First(head, head.ID).Error; err != nil {
		return nil, err
	}
	return head, nil
}

func (s *OrderService) rebuildManagedRuntime(ctx context.Context) error {
	if err := s.xray.RebuildConfigFile(ctx); err != nil {
		return err
	}
	return s.xray.RestartManaged()
}

func (s *OrderService) updateOrderGroup(ctx context.Context, head model.Order, in UpdateOrderInput) error {
	now := time.Now()
	targetName := strings.TrimSpace(head.Name)
	if strings.TrimSpace(in.Name) != "" {
		targetName = strings.TrimSpace(in.Name)
	}
	targetExpires := head.ExpiresAt
	if !in.ExpiresAt.IsZero() {
		targetExpires = in.ExpiresAt
	}
	targetStatus := model.OrderStatusActive
	itemStatus := model.OrderItemStatusActive
	if !targetExpires.After(now) {
		targetStatus = model.OrderStatusExpired
		itemStatus = model.OrderItemStatusExpired
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		children, err := s.loadGroupChildrenTx(tx, head.ID)
		if err != nil {
			return err
		}

		targetEntryID := head.DedicatedEntryID
		targetPort := head.Port
		if head.Mode == model.OrderModeDedicated && in.DedicatedEntryID > 0 {
			entry := model.DedicatedEntry{}
			if err := tx.Where("id = ? and enabled = 1", in.DedicatedEntryID).First(&entry).Error; err != nil {
				return fmt.Errorf("dedicated entry invalid: %w", err)
			}
			port := chooseDedicatedPrimaryPort(entry)
			if port <= 0 {
				return errors.New("dedicated entry has no usable protocol port")
			}
			targetPort = port
			targetEntryID = &entry.ID
		}

		if strings.TrimSpace(in.DedicatedEgressLines) != "" {
			if err := s.updateGroupSocks5Tx(tx, head, children, in.DedicatedEgressLines, now); err != nil {
				return err
			}
		}
		if in.RegenerateDedicatedCredentials || strings.TrimSpace(in.DedicatedCredentialLines) != "" {
			if err := s.updateGroupCredentialsTx(tx, head, children, in.DedicatedCredentialLines, in.RegenerateDedicatedCredentials, now); err != nil {
				return err
			}
		}

		headUpdates := map[string]interface{}{
			"name":                targetName,
			"status":              targetStatus,
			"expires_at":          targetExpires,
			"notify_one_day_sent": false,
			"notify_expired_sent": false,
			"port":                targetPort,
			"updated_at":          now,
		}
		if targetEntryID != nil {
			headUpdates["dedicated_entry_id"] = *targetEntryID
		}
		if err := tx.Model(&model.Order{}).Where("id = ?", head.ID).Updates(headUpdates).Error; err != nil {
			return err
		}
		for _, child := range children {
			childUpdates := map[string]interface{}{
				"status":              targetStatus,
				"expires_at":          targetExpires,
				"notify_one_day_sent": false,
				"notify_expired_sent": false,
				"port":                targetPort,
				"updated_at":          now,
			}
			if targetEntryID != nil {
				childUpdates["dedicated_entry_id"] = *targetEntryID
			}
			if err := tx.Model(&model.Order{}).Where("id = ?", child.ID).Updates(childUpdates).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.OrderItem{}).Where("order_id = ?", child.ID).Updates(map[string]interface{}{
				"status":     itemStatus,
				"port":       targetPort,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) deactivateOrderGroup(ctx context.Context, head model.Order, status string, itemStatus string) error {
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).Where("id = ? or parent_order_id = ?", head.ID, head.ID).Updates(map[string]interface{}{
			"status":     status,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.OrderItem{}).Where("order_id in (?)", tx.Model(&model.Order{}).Select("id").Where("id = ? or parent_order_id = ?", head.ID, head.ID)).Updates(map[string]interface{}{
			"status":     itemStatus,
			"updated_at": now,
		}).Error
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) renewOrderGroup(ctx context.Context, head model.Order, moreDays int) error {
	if moreDays <= 0 {
		moreDays = 30
	}
	base := head.ExpiresAt
	if base.Before(time.Now()) {
		base = time.Now()
	}
	newExpires := base.Add(time.Duration(moreDays) * 24 * time.Hour)
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).Where("id = ? or parent_order_id = ?", head.ID, head.ID).Updates(map[string]interface{}{
			"status":              model.OrderStatusActive,
			"expires_at":          newExpires,
			"notify_one_day_sent": false,
			"notify_expired_sent": false,
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.OrderItem{}).Where("order_id in (?)", tx.Model(&model.Order{}).Select("id").Where("id = ? or parent_order_id = ?", head.ID, head.ID)).Updates(map[string]interface{}{
			"status":     model.OrderItemStatusActive,
			"updated_at": now,
		}).Error
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) loadGroupChildrenTx(tx *gorm.DB, headID uint) ([]model.Order, error) {
	rows := []model.Order{}
	err := tx.Preload("Items").Where("parent_order_id = ?", headID).Order("sequence_no asc, id asc").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *OrderService) SplitOrder(ctx context.Context, orderID uint) ([]model.Order, error) {
	head := model.Order{}
	if err := s.db.Preload("Items").First(&head, orderID).Error; err != nil {
		return nil, err
	}
	if head.ParentOrderID != nil {
		return nil, errors.New("child order cannot be split")
	}
	if head.IsGroupHead {
		children := []model.Order{}
		if err := s.db.Where("parent_order_id = ?", head.ID).Order("sequence_no asc, id asc").Find(&children).Error; err != nil {
			return nil, err
		}
		if len(children) > 0 {
			return children, nil
		}
	}
	if len(head.Items) <= 1 {
		return nil, errors.New("order has less than 2 items, split skipped")
	}

	now := time.Now()
	children := []model.Order{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		sort.Slice(head.Items, func(i, j int) bool { return head.Items[i].ID < head.Items[j].ID })
		if err := tx.Model(&model.Order{}).Where("id = ?", head.ID).Updates(map[string]interface{}{
			"is_group_head": true,
			"group_id":      head.ID,
			"sequence_no":   0,
			"quantity":      len(head.Items),
			"updated_at":    now,
		}).Error; err != nil {
			return err
		}
		oldItemIDs := make([]uint, 0, len(head.Items))
		for i, item := range head.Items {
			parentID := head.ID
			child := model.Order{
				CustomerID:       head.CustomerID,
				GroupID:          head.ID,
				ParentOrderID:    &parentID,
				IsGroupHead:      false,
				SequenceNo:       i + 1,
				DedicatedEntryID: head.DedicatedEntryID,
				Name:             fmt.Sprintf("%s-%03d", head.Name, i+1),
				Mode:             head.Mode,
				Status:           head.Status,
				Quantity:         1,
				Port:             head.Port,
				StartsAt:         head.StartsAt,
				ExpiresAt:        head.ExpiresAt,
			}
			if err := tx.Create(&child).Error; err != nil {
				return err
			}
			newItem := item
			newItem.ID = 0
			newItem.OrderID = child.ID
			newItem.CreatedAt = now
			newItem.UpdatedAt = now
			if err := tx.Create(&newItem).Error; err != nil {
				return err
			}
			if strings.EqualFold(strings.TrimSpace(head.Mode), model.OrderModeDedicated) && strings.TrimSpace(item.ForwardAddress) != "" && item.ForwardPort > 0 {
				egress := model.DedicatedEgress{
					OrderID:     child.ID,
					OrderItemID: newItem.ID,
					Address:     item.ForwardAddress,
					Port:        item.ForwardPort,
					Username:    item.ForwardUsername,
					Password:    item.ForwardPassword,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				if err := tx.Where("order_item_id = ?", item.ID).Delete(&model.DedicatedEgress{}).Error; err != nil {
					return err
				}
				if err := tx.Create(&egress).Error; err != nil {
					return err
				}
			}
			children = append(children, child)
			oldItemIDs = append(oldItemIDs, item.ID)
		}
		if len(oldItemIDs) > 0 {
			if err := tx.Where("order_item_id in ?", oldItemIDs).Delete(&model.XrayResource{}).Error; err != nil {
				return err
			}
			if err := tx.Where("order_item_id in ?", oldItemIDs).Delete(&model.DedicatedEgress{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id in ?", oldItemIDs).Delete(&model.OrderItem{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.rebuildManagedRuntime(ctx); err != nil {
		s.log.Warn("sync runtime after split failed", zap.Error(err), zap.Uint("order_id", orderID))
	}
	rows := []model.Order{}
	if err := s.db.Preload("Items").Where("parent_order_id = ?", orderID).Order("sequence_no asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *OrderService) UpdateGroupSocks5(ctx context.Context, orderID uint, lines string) error {
	head := model.Order{}
	if err := s.db.First(&head, orderID).Error; err != nil {
		return err
	}
	if !head.IsGroupHead {
		return errors.New("only group head order can batch update socks5")
	}
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		children, err := s.loadGroupChildrenTx(tx, head.ID)
		if err != nil {
			return err
		}
		return s.updateGroupSocks5Tx(tx, head, children, lines, now)
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) UpdateGroupCredentials(ctx context.Context, orderID uint, lines string, regenerate bool) error {
	head := model.Order{}
	if err := s.db.First(&head, orderID).Error; err != nil {
		return err
	}
	if !head.IsGroupHead {
		return errors.New("only group head order can batch update credentials")
	}
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		children, err := s.loadGroupChildrenTx(tx, head.ID)
		if err != nil {
			return err
		}
		return s.updateGroupCredentialsTx(tx, head, children, lines, regenerate, now)
	}); err != nil {
		return err
	}
	return s.rebuildManagedRuntime(ctx)
}

func (s *OrderService) updateGroupSocks5Tx(tx *gorm.DB, head model.Order, children []model.Order, lines string, now time.Time) error {
	_ = head
	egRows, err := parseDedicatedEgressLines(lines)
	if err != nil {
		return err
	}
	if len(egRows) != len(children) {
		return fmt.Errorf("lines count %d not equal child orders %d", len(egRows), len(children))
	}
	for i, child := range children {
		if len(child.Items) == 0 {
			return fmt.Errorf("child order %d has no item", child.ID)
		}
		item := child.Items[0]
		eg := egRows[i]
		if err := tx.Model(&model.OrderItem{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
			"outbound_type":     model.OutboundTypeSocks5,
			"forward_address":   eg.Address,
			"forward_port":      eg.Port,
			"forward_username":  eg.Username,
			"forward_password":  eg.Password,
			"socks_outbound_id": nil,
			"updated_at":        now,
		}).Error; err != nil {
			return err
		}
		egress := model.DedicatedEgress{}
		err := tx.Where("order_item_id = ?", item.ID).First(&egress).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			egress = model.DedicatedEgress{OrderID: child.ID, OrderItemID: item.ID, CreatedAt: now}
		}
		egress.Address = eg.Address
		egress.Port = eg.Port
		egress.Username = eg.Username
		egress.Password = eg.Password
		egress.UpdatedAt = now
		if err := tx.Save(&egress).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *OrderService) updateGroupCredentialsTx(tx *gorm.DB, head model.Order, children []model.Order, lines string, regenerate bool, now time.Time) error {
	_ = head
	credRows := []DedicatedCredentialLine{}
	var err error
	if regenerate {
		credRows = make([]DedicatedCredentialLine, 0, len(children))
		for range children {
			credRows = append(credRows, DedicatedCredentialLine{
				Username: randomString(8),
				Password: randomString(12),
				UUID:     randomUUID(),
			})
		}
	} else {
		credRows, err = parseDedicatedCredentialLines(lines)
		if err != nil {
			return err
		}
	}
	if len(credRows) != len(children) {
		return fmt.Errorf("credential count %d not equal child orders %d", len(credRows), len(children))
	}
	for i, child := range children {
		if len(child.Items) == 0 {
			return fmt.Errorf("child order %d has no item", child.ID)
		}
		item := child.Items[0]
		cred := credRows[i]
		if cred.UUID == "" {
			cred.UUID = randomUUID()
		}
		if err := tx.Model(&model.OrderItem{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
			"username":   cred.Username,
			"password":   cred.Password,
			"vmess_uuid": cred.UUID,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
