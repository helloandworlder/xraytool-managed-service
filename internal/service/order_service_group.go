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
	protocol := strings.TrimSpace(in.DedicatedProtocol)
	if protocol == "" {
		protocol = model.DedicatedFeatureMixed
	}
	protocol, err := normalizeDedicatedProtocol(protocol)
	if err != nil {
		return nil, err
	}
	inbound, ingress, entry, err := s.resolveDedicatedBindingForCreate(in, protocol)
	if err != nil {
		return nil, err
	}
	primaryPort := inbound.ListenPort
	if primaryPort <= 0 {
		return nil, fmt.Errorf("dedicated inbound has no usable port for protocol %s", protocol)
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
		customer := model.Customer{}
		if err := s.db.First(&customer, in.CustomerID).Error; err == nil {
			code := strings.TrimSpace(customer.Code)
			if code == "" {
				code = fmt.Sprintf("C%d", customer.ID)
			}
			baseName = fmt.Sprintf("%s-%s-%s", code, protocol, now.Format("20060102150405"))
		} else {
			baseName = fmt.Sprintf("Dedicated-%d-%s", in.CustomerID, now.Format("20060102150405"))
		}
	}

	head := &model.Order{
		CustomerID:         in.CustomerID,
		Name:               baseName,
		Mode:               model.OrderModeDedicated,
		DedicatedProtocol:  protocol,
		Status:             model.OrderStatusActive,
		Quantity:           len(egressRows),
		Port:               primaryPort,
		StartsAt:           now,
		ExpiresAt:          expiresAt,
		IsGroupHead:        true,
		DedicatedEntryID:   uintPtrOrNil(entry.ID),
		DedicatedInboundID: uintPtrOrNil(inbound.ID),
		DedicatedIngressID: uintPtrOrNil(ingress.ID),
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(head).Error; err != nil {
			return err
		}
		if strings.TrimSpace(in.Name) == "" {
			head.Name = fmt.Sprintf("%s-%d", baseName, head.ID)
			if err := tx.Model(&model.Order{}).Where("id = ?", head.ID).Updates(map[string]interface{}{"name": head.Name, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Order{}).Where("id = ?", head.ID).Updates(map[string]interface{}{
			"group_id":   head.ID,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		for i, outbound := range egressRows {
			childName := fmt.Sprintf("%s-%03d", head.Name, i+1)
			seq := i + 1
			parentID := head.ID
			itemUser, itemPass, itemUUID := generateDedicatedCredentialsByProtocol(protocol)
			child := model.Order{
				CustomerID:         in.CustomerID,
				GroupID:            head.ID,
				ParentOrderID:      &parentID,
				SequenceNo:         seq,
				DedicatedEntryID:   uintPtrOrNil(entry.ID),
				DedicatedInboundID: uintPtrOrNil(inbound.ID),
				DedicatedIngressID: uintPtrOrNil(ingress.ID),
				DedicatedProtocol:  protocol,
				Name:               childName,
				Mode:               model.OrderModeDedicated,
				Status:             model.OrderStatusActive,
				Quantity:           1,
				Port:               primaryPort,
				StartsAt:           now,
				ExpiresAt:          expiresAt,
			}
			if err := tx.Create(&child).Error; err != nil {
				return err
			}
			item := model.OrderItem{
				OrderID:         child.ID,
				IP:              "127.0.0.1",
				Port:            primaryPort,
				Username:        itemUser,
				Password:        itemPass,
				VmessUUID:       itemUUID,
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
			fillDedicatedEgressProbe(&egress)
			if err := tx.Create(&egress).Error; err != nil {
				return err
			}
			country := normalizeCountryPrefix(egress.CountryCode)
			childFinalName := fmt.Sprintf("%s-%s-%03d", head.Name, country, seq)
			if err := tx.Model(&model.Order{}).Where("id = ?", child.ID).Updates(map[string]interface{}{"name": childFinalName, "updated_at": now}).Error; err != nil {
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

	if err := s.db.Preload("Customer").Preload("DedicatedEntry").Preload("DedicatedInbound").Preload("DedicatedIngress").Preload("Items").First(head, head.ID).Error; err != nil {
		return nil, err
	}
	return head, nil
}

func uintPtrOrNil(v uint) *uint {
	if v == 0 {
		return nil
	}
	vv := v
	return &vv
}

func (s *OrderService) resolveDedicatedBindingForCreate(in CreateOrderInput, protocol string) (model.DedicatedInbound, model.DedicatedIngress, model.DedicatedEntry, error) {
	inbound := model.DedicatedInbound{}
	ingress := model.DedicatedIngress{}
	entry := model.DedicatedEntry{}
	if in.DedicatedInboundID > 0 && in.DedicatedIngressID > 0 {
		if err := s.db.Where("id = ? and enabled = 1", in.DedicatedInboundID).First(&inbound).Error; err != nil {
			return inbound, ingress, entry, fmt.Errorf("dedicated inbound invalid: %w", err)
		}
		if !strings.EqualFold(strings.TrimSpace(inbound.Protocol), protocol) {
			return inbound, ingress, entry, fmt.Errorf("inbound protocol mismatch, expect %s got %s", protocol, inbound.Protocol)
		}
		if err := s.db.Where("id = ? and dedicated_inbound_id = ? and enabled = 1", in.DedicatedIngressID, inbound.ID).First(&ingress).Error; err != nil {
			return inbound, ingress, entry, fmt.Errorf("dedicated ingress invalid: %w", err)
		}
		if in.DedicatedEntryID > 0 {
			_ = s.db.Where("id = ?", in.DedicatedEntryID).First(&entry).Error
		}
		return inbound, ingress, entry, nil
	}
	if in.DedicatedEntryID == 0 {
		return inbound, ingress, entry, errors.New("dedicated_inbound_id + dedicated_ingress_id required")
	}
	if err := s.db.Where("id = ? and enabled = 1", in.DedicatedEntryID).First(&entry).Error; err != nil {
		return inbound, ingress, entry, fmt.Errorf("dedicated entry invalid: %w", err)
	}
	if !hasDedicatedFeature(entry.Features, protocol) {
		return inbound, ingress, entry, fmt.Errorf("dedicated entry does not support protocol %s", protocol)
	}
	port := dedicatedPortByProtocol(entry, protocol)
	if port <= 0 {
		return inbound, ingress, entry, fmt.Errorf("dedicated entry has no usable port for protocol %s", protocol)
	}
	if err := s.db.Where("protocol = ? and listen_port = ?", protocol, port).First(&inbound).Error; err != nil {
		return inbound, ingress, entry, errors.New("missing migrated inbound for legacy dedicated entry")
	}
	if err := s.db.Where("dedicated_inbound_id = ? and domain = ? and ingress_port = ?", inbound.ID, entry.Domain, port).First(&ingress).Error; err != nil {
		return inbound, ingress, entry, errors.New("missing migrated ingress for legacy dedicated entry")
	}
	return inbound, ingress, entry, nil
}

func (s *OrderService) resolveDedicatedBindingForUpdateTx(tx *gorm.DB, protocol string, entryID *uint, inboundID *uint, ingressID *uint) (*uint, *uint, *uint, int, error) {
	protocol, err := normalizeDedicatedProtocol(protocol)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	entry := model.DedicatedEntry{}
	hasEntry := entryID != nil && *entryID > 0
	if hasEntry {
		if err := tx.Where("id = ?", *entryID).First(&entry).Error; err != nil {
			return nil, nil, nil, 0, fmt.Errorf("dedicated entry invalid: %w", err)
		}
		if !hasDedicatedFeature(entry.Features, protocol) {
			return nil, nil, nil, 0, fmt.Errorf("dedicated entry does not support protocol %s", protocol)
		}
	}

	inbound := model.DedicatedInbound{}
	hasInbound := inboundID != nil && *inboundID > 0
	if hasInbound {
		if err := tx.Where("id = ?", *inboundID).First(&inbound).Error; err != nil {
			return nil, nil, nil, 0, fmt.Errorf("dedicated inbound invalid: %w", err)
		}
		if !strings.EqualFold(strings.TrimSpace(inbound.Protocol), protocol) {
			return nil, nil, nil, 0, fmt.Errorf("inbound protocol mismatch, expect %s got %s", protocol, inbound.Protocol)
		}
	} else {
		if !hasEntry {
			return nil, nil, nil, 0, errors.New("dedicated_inbound_id + dedicated_ingress_id required")
		}
		port := dedicatedPortByProtocol(entry, protocol)
		if port <= 0 {
			return nil, nil, nil, 0, fmt.Errorf("dedicated entry has no usable port for protocol %s", protocol)
		}
		if err := tx.Where("protocol = ? and listen_port = ?", protocol, port).First(&inbound).Error; err != nil {
			return nil, nil, nil, 0, errors.New("missing migrated inbound for legacy dedicated entry")
		}
	}

	if inbound.ListenPort <= 0 {
		return nil, nil, nil, 0, fmt.Errorf("dedicated inbound has no usable port for protocol %s", protocol)
	}

	ingress := model.DedicatedIngress{}
	hasIngress := ingressID != nil && *ingressID > 0
	if hasIngress {
		if err := tx.Where("id = ? and dedicated_inbound_id = ?", *ingressID, inbound.ID).First(&ingress).Error; err != nil {
			return nil, nil, nil, 0, fmt.Errorf("dedicated ingress invalid: %w", err)
		}
	} else {
		if !hasEntry {
			return nil, nil, nil, 0, errors.New("dedicated_ingress_id is required")
		}
		if err := tx.Where("dedicated_inbound_id = ? and domain = ? and ingress_port = ?", inbound.ID, entry.Domain, inbound.ListenPort).First(&ingress).Error; err != nil {
			return nil, nil, nil, 0, errors.New("missing migrated ingress for legacy dedicated entry")
		}
	}

	resolvedEntryID := (*uint)(nil)
	if hasEntry {
		resolvedEntryID = uintPtrOrNil(entry.ID)
	}
	resolvedInboundID := uintPtrOrNil(inbound.ID)
	resolvedIngressID := uintPtrOrNil(ingress.ID)
	return resolvedEntryID, resolvedInboundID, resolvedIngressID, inbound.ListenPort, nil
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
		targetInboundID := head.DedicatedInboundID
		targetIngressID := head.DedicatedIngressID
		targetProtocol := strings.TrimSpace(head.DedicatedProtocol)
		if targetProtocol == "" {
			targetProtocol = model.DedicatedFeatureMixed
		}
		if strings.TrimSpace(in.DedicatedProtocol) != "" {
			targetProtocol, err = normalizeDedicatedProtocol(in.DedicatedProtocol)
			if err != nil {
				return err
			}
		}
		targetPort := head.Port
		if head.Mode == model.OrderModeDedicated {
			if in.DedicatedEntryID > 0 {
				targetEntryID = &in.DedicatedEntryID
			}
			if in.DedicatedInboundID > 0 {
				targetInboundID = &in.DedicatedInboundID
			}
			if in.DedicatedIngressID > 0 {
				targetIngressID = &in.DedicatedIngressID
			}
			resolvedEntryID, resolvedInboundID, resolvedIngressID, resolvedPort, resolveErr := s.resolveDedicatedBindingForUpdateTx(tx, targetProtocol, targetEntryID, targetInboundID, targetIngressID)
			if resolveErr != nil {
				return resolveErr
			}
			targetEntryID = resolvedEntryID
			targetInboundID = resolvedInboundID
			targetIngressID = resolvedIngressID
			targetPort = resolvedPort
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
			"dedicated_protocol":  targetProtocol,
			"port":                targetPort,
			"updated_at":          now,
		}
		if targetEntryID != nil {
			headUpdates["dedicated_entry_id"] = *targetEntryID
		}
		if targetInboundID != nil {
			headUpdates["dedicated_inbound_id"] = *targetInboundID
		}
		if targetIngressID != nil {
			headUpdates["dedicated_ingress_id"] = *targetIngressID
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
				"dedicated_protocol":  targetProtocol,
				"port":                targetPort,
				"updated_at":          now,
			}
			if targetEntryID != nil {
				childUpdates["dedicated_entry_id"] = *targetEntryID
			}
			if targetInboundID != nil {
				childUpdates["dedicated_inbound_id"] = *targetInboundID
			}
			if targetIngressID != nil {
				childUpdates["dedicated_ingress_id"] = *targetIngressID
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

func (s *OrderService) RenewOrderGroupSelected(ctx context.Context, headOrderID uint, childOrderIDs []uint, moreDays int) error {
	if headOrderID == 0 {
		return errors.New("order_id is required")
	}
	ids := uniqueUintIDs(childOrderIDs)
	if len(ids) == 0 {
		return errors.New("child_order_ids is empty")
	}
	if moreDays <= 0 {
		moreDays = 30
	}

	head := model.Order{}
	if err := s.db.First(&head, headOrderID).Error; err != nil {
		return err
	}
	if !head.IsGroupHead {
		return errors.New("only group head order supports selected renew")
	}

	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		children := []model.Order{}
		if err := tx.Where("parent_order_id = ? and id in ?", head.ID, ids).Find(&children).Error; err != nil {
			return err
		}
		if len(children) == 0 {
			return errors.New("no matched child orders")
		}
		if len(children) != len(ids) {
			return fmt.Errorf("matched child orders %d not equal request %d", len(children), len(ids))
		}

		for _, child := range children {
			base := child.ExpiresAt
			if base.Before(now) {
				base = now
			}
			newExpires := base.Add(time.Duration(moreDays) * 24 * time.Hour)
			if err := tx.Model(&model.Order{}).Where("id = ?", child.ID).Updates(map[string]interface{}{
				"status":              model.OrderStatusActive,
				"expires_at":          newExpires,
				"notify_one_day_sent": false,
				"notify_expired_sent": false,
				"updated_at":          now,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.OrderItem{}).Where("order_id = ?", child.ID).Updates(map[string]interface{}{
				"status":     model.OrderItemStatusActive,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		}

		allChildren := []model.Order{}
		if err := tx.Select("id", "status", "expires_at").Where("parent_order_id = ?", head.ID).Find(&allChildren).Error; err != nil {
			return err
		}
		headExpires := now
		headStatus := model.OrderStatusExpired
		for _, child := range allChildren {
			if child.ExpiresAt.After(headExpires) {
				headExpires = child.ExpiresAt
			}
			if child.Status == model.OrderStatusActive && child.ExpiresAt.After(now) {
				headStatus = model.OrderStatusActive
			}
		}
		if err := tx.Model(&model.Order{}).Where("id = ?", head.ID).Updates(map[string]interface{}{
			"status":              headStatus,
			"expires_at":          headExpires,
			"notify_one_day_sent": false,
			"notify_expired_sent": false,
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		return nil
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
				CustomerID:         head.CustomerID,
				GroupID:            head.ID,
				ParentOrderID:      &parentID,
				IsGroupHead:        false,
				SequenceNo:         i + 1,
				DedicatedEntryID:   head.DedicatedEntryID,
				DedicatedInboundID: head.DedicatedInboundID,
				DedicatedIngressID: head.DedicatedIngressID,
				DedicatedProtocol:  head.DedicatedProtocol,
				Name:               fmt.Sprintf("%s-%03d", head.Name, i+1),
				Mode:               head.Mode,
				Status:             head.Status,
				Quantity:           1,
				Port:               head.Port,
				StartsAt:           head.StartsAt,
				ExpiresAt:          head.ExpiresAt,
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
				fillDedicatedEgressProbe(&egress)
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
		fillDedicatedEgressProbe(&egress)
		egress.UpdatedAt = now
		if err := tx.Save(&egress).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *OrderService) updateGroupCredentialsTx(tx *gorm.DB, head model.Order, children []model.Order, lines string, regenerate bool, now time.Time) error {
	protocol := strings.TrimSpace(head.DedicatedProtocol)
	if protocol == "" {
		protocol = model.DedicatedFeatureMixed
	}
	credRows := []DedicatedCredentialLine{}
	var err error
	if regenerate {
		credRows = make([]DedicatedCredentialLine, 0, len(children))
		for range children {
			user, pass, uuid := generateDedicatedCredentialsByProtocol(protocol)
			credRows = append(credRows, DedicatedCredentialLine{
				Username: user,
				Password: pass,
				UUID:     uuid,
			})
		}
	} else {
		credRows, err = parseDedicatedCredentialLinesForProtocol(lines, protocol)
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
		updates := map[string]interface{}{"updated_at": now}
		switch protocol {
		case model.DedicatedFeatureMixed:
			if strings.TrimSpace(cred.Username) == "" {
				cred.Username = randomString(8)
			}
			if strings.TrimSpace(cred.Password) == "" {
				cred.Password = randomString(12)
			}
			if strings.TrimSpace(cred.UUID) == "" {
				cred.UUID = randomUUID()
			}
			updates["username"] = cred.Username
			updates["password"] = cred.Password
			updates["vmess_uuid"] = cred.UUID
		case model.DedicatedFeatureVmess, model.DedicatedFeatureVless:
			if strings.TrimSpace(cred.UUID) == "" {
				cred.UUID = randomUUID()
			}
			updates["vmess_uuid"] = cred.UUID
		case model.DedicatedFeatureShadowsocks:
			if strings.TrimSpace(cred.Password) == "" {
				cred.Password = randomString(12)
			}
			updates["password"] = cred.Password
		}
		if err := tx.Model(&model.OrderItem{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func generateDedicatedCredentialsByProtocol(protocol string) (string, string, string) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	username := randomString(8)
	password := randomString(12)
	uuid := randomUUID()
	switch protocol {
	case model.DedicatedFeatureVmess, model.DedicatedFeatureVless:
		return username, password, uuid
	case model.DedicatedFeatureShadowsocks:
		return username, password, uuid
	default:
		return username, password, uuid
	}
}

func fillDedicatedEgressProbe(row *model.DedicatedEgress) {
	if row == nil {
		return
	}
	now := time.Now()
	exitIP, country, err := probeSocksOutbound(row.Address, row.Port, row.Username, row.Password)
	row.LastProbedAt = &now
	if err != nil {
		row.ProbeStatus = "failed"
		row.ProbeError = err.Error()
		row.ExitIP = ""
		row.CountryCode = ""
		return
	}
	row.ProbeStatus = "ok"
	row.ProbeError = ""
	row.ExitIP = strings.TrimSpace(exitIP)
	row.CountryCode = strings.ToLower(strings.TrimSpace(country))
}
