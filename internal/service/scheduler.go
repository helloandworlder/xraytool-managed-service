package service

import (
	"context"
	"fmt"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Scheduler struct {
	db       *gorm.DB
	orders   *OrderService
	bark     *BarkService
	logger   *zap.Logger
	interval time.Duration
}

func NewScheduler(db *gorm.DB, orders *OrderService, bark *BarkService, logger *zap.Logger, interval time.Duration) *Scheduler {
	return &Scheduler{db: db, orders: orders, bark: bark, logger: logger, interval: interval}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	now := time.Now()
	oneDayLater := now.Add(24 * time.Hour)

	var remindOrders []model.Order
	if err := s.db.Where("status = ? and expires_at > ? and expires_at <= ? and notify_one_day_sent = 0", model.OrderStatusActive, now, oneDayLater).
		Find(&remindOrders).Error; err == nil {
		for _, order := range remindOrders {
			title := "XrayTool 订单将到期"
			body := fmt.Sprintf("订单[%s] 将于 %s 到期", order.Name, order.ExpiresAt.Format("2006-01-02 15:04:05"))
			if err := s.bark.Notify(title, body); err != nil {
				s.logger.Warn("bark one-day notify failed", zap.Error(err), zap.Uint("order_id", order.ID))
				continue
			}
			_ = s.db.Model(&model.Order{}).Where("id = ?", order.ID).Update("notify_one_day_sent", true).Error
		}
	}

	var expired []model.Order
	if err := s.db.Where("status = ? and expires_at <= ?", model.OrderStatusActive, now).Find(&expired).Error; err == nil {
		for _, order := range expired {
			if err := s.orders.DeactivateOrder(ctx, order.ID, model.OrderStatusExpired); err != nil {
				s.logger.Warn("expire order deactivate failed", zap.Error(err), zap.Uint("order_id", order.ID))
				continue
			}
		}
	}

	var expiredNotified []model.Order
	if err := s.db.Where("status = ? and notify_expired_sent = 0", model.OrderStatusExpired).Find(&expiredNotified).Error; err == nil {
		for _, order := range expiredNotified {
			title := "XrayTool 订单已到期"
			body := fmt.Sprintf("订单[%s] 已到期并自动下线", order.Name)
			if err := s.bark.Notify(title, body); err != nil {
				s.logger.Warn("bark expired notify failed", zap.Error(err), zap.Uint("order_id", order.ID))
				continue
			}
			_ = s.db.Model(&model.Order{}).Where("id = ?", order.ID).Update("notify_expired_sent", true).Error
		}
	}
}
