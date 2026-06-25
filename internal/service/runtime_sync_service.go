package service

import (
	"context"
	"strings"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const runtimeSyncWorkerInterval = 2 * time.Second
const runtimeSyncTaskTimeout = 2 * time.Minute

type ListRuntimeSyncTasksInput struct {
	Status string
	Limit  int
}

func (s *OrderService) StartRuntimeSyncWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(runtimeSyncWorkerInterval)
		defer ticker.Stop()
		for {
			if err := s.processNextRuntimeSyncTask(ctx); err != nil {
				s.log.Warn("runtime sync worker failed", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *OrderService) ListRuntimeSyncTasks(in ListRuntimeSyncTasksInput) ([]model.RuntimeSyncTask, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := s.db.Model(&model.RuntimeSyncTask{})
	if status := strings.TrimSpace(in.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	rows := make([]model.RuntimeSyncTask, 0)
	if err := query.Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *OrderService) RetryRuntimeSyncTask(id uint) (*model.RuntimeSyncTask, error) {
	var task model.RuntimeSyncTask
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.db.Model(&model.RuntimeSyncTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       model.RuntimeSyncStatusPending,
		"error":        "",
		"requested_at": now,
		"started_at":   nil,
		"finished_at":  nil,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *OrderService) enqueueRuntimeSyncTask(reason string, orderID *uint) error {
	now := time.Now()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "order_change"
	}
	task := model.RuntimeSyncTask{
		Status:      model.RuntimeSyncStatusPending,
		Reason:      reason,
		OrderID:     orderID,
		RequestedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.db.Create(&task).Error
}

func (s *OrderService) processNextRuntimeSyncTask(ctx context.Context) error {
	var task model.RuntimeSyncTask
	err := s.db.
		Where("status = ?", model.RuntimeSyncStatusPending).
		Order("id asc").
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	now := time.Now()
	if err := s.db.Model(&model.RuntimeSyncTask{}).Where("id = ? and status = ?", task.ID, model.RuntimeSyncStatusPending).Updates(map[string]interface{}{
		"status":     model.RuntimeSyncStatusRunning,
		"attempts":   task.Attempts + 1,
		"started_at": now,
		"updated_at": now,
	}).Error; err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, runtimeSyncTaskTimeout)
	err = s.rebuildManagedRuntimeNow(runCtx)
	cancel()

	finishedAt := time.Now()
	if err != nil {
		s.log.Warn("runtime sync task failed", zap.Uint("task_id", task.ID), zap.Error(err))
		return s.db.Model(&model.RuntimeSyncTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"status":      model.RuntimeSyncStatusFailed,
			"error":       err.Error(),
			"finished_at": finishedAt,
			"updated_at":  finishedAt,
		}).Error
	}

	return s.db.Model(&model.RuntimeSyncTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":      model.RuntimeSyncStatusSuccess,
		"error":       "",
		"finished_at": finishedAt,
		"updated_at":  finishedAt,
	}).Error
}
