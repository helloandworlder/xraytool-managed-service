package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	runtimeInstanceUplinkLimitKey   = "runtime_instance_uplink_limit_bps"
	runtimeInstanceDownlinkLimitKey = "runtime_instance_downlink_limit_bps"
	runtimeLimitPolicyVersionKey    = "runtime_limit_policy_version"
)

type RuntimeLimitPolicyInput struct {
	UplinkLimitBps   *uint64                  `json:"uplinkLimitBps,omitempty"`
	DownlinkLimitBps *uint64                  `json:"downlinkLimitBps,omitempty"`
	Users            []RuntimeUserLimitPolicy `json:"users,omitempty"`
}

type RuntimeUserLimitPolicy struct {
	Username         string `json:"username"`
	UplinkLimitBps   uint64 `json:"uplinkLimitBps"`
	DownlinkLimitBps uint64 `json:"downlinkLimitBps"`
	MaxConnections   uint64 `json:"maxConnections,omitempty"`
}

type RuntimeLimitPolicyValues struct {
	UplinkLimitBps   uint64 `json:"uplinkLimitBps"`
	DownlinkLimitBps uint64 `json:"downlinkLimitBps"`
	PolicyVersion    uint64 `json:"policyVersion"`
}

// ReapplyLimitPolicyRuntime persists the desired instance policy before
// enqueueing work. The worker later marks the task applied only after Xray's
// fork accepts the direction-specific runtime command.
func (s *OrderService) ReapplyLimitPolicyRuntime(ctx context.Context, in RuntimeLimitPolicyInput) (*model.RuntimeSyncTask, error) {
	now := time.Now()
	var desired RuntimeLimitPolicyValues
	var created model.RuntimeSyncTask
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		current, err := loadRuntimeLimitPolicyValues(ctx, tx, 0, 0)
		if err != nil {
			return err
		}
		desired = current
		if in.UplinkLimitBps != nil {
			desired.UplinkLimitBps = *in.UplinkLimitBps
		}
		if in.DownlinkLimitBps != nil {
			desired.DownlinkLimitBps = *in.DownlinkLimitBps
		}
		desired.PolicyVersion++
		for key, value := range map[string]string{
			runtimeInstanceUplinkLimitKey:   strconv.FormatUint(desired.UplinkLimitBps, 10),
			runtimeInstanceDownlinkLimitKey: strconv.FormatUint(desired.DownlinkLimitBps, 10),
			runtimeLimitPolicyVersionKey:    strconv.FormatUint(desired.PolicyVersion, 10),
		} {
			row := model.Setting{Key: key, Value: value, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		for _, userPolicy := range normalizeRuntimeUserPolicies(in.Users) {
			if userPolicy.Username == "" {
				continue
			}
			if userPolicy.UplinkLimitBps > uint64(^uint64(0)>>1) || userPolicy.DownlinkLimitBps > uint64(^uint64(0)>>1) || userPolicy.MaxConnections > uint64(^uint64(0)>>1) {
				return fmt.Errorf("runtime user policy %s exceeds int64 storage", userPolicy.Username)
			}
			if err := tx.Model(&model.OrderItem{}).
				Where("username = ? and status = ?", userPolicy.Username, model.OrderItemStatusActive).
				Updates(map[string]interface{}{
					"uplink_limit_bps":   int64(userPolicy.UplinkLimitBps),
					"downlink_limit_bps": int64(userPolicy.DownlinkLimitBps),
					"max_connections":    int64(userPolicy.MaxConnections),
				}).Error; err != nil {
				return err
			}
		}
		created = model.RuntimeSyncTask{
			Status:                  model.RuntimeSyncStatusPending,
			Reason:                  "limit_policy_reapply",
			Target:                  "managed_xray_instance",
			PolicyVersion:           desired.PolicyVersion,
			DesiredUplinkLimitBps:   desired.UplinkLimitBps,
			DesiredDownlinkLimitBps: desired.DownlinkLimitBps,
			RequestedAt:             now,
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		return tx.Create(&created).Error
	}); err != nil {
		return nil, err
	}
	return &created, nil
}

func normalizeRuntimeUserPolicies(input []RuntimeUserLimitPolicy) []RuntimeUserLimitPolicy {
	byUsername := make(map[string]RuntimeUserLimitPolicy, len(input))
	for _, item := range input {
		username := strings.TrimSpace(item.Username)
		if username == "" {
			continue
		}
		item.Username = username
		current, ok := byUsername[username]
		if !ok {
			byUsername[username] = item
			continue
		}
		current.UplinkLimitBps = strictestPositiveUint64(current.UplinkLimitBps, item.UplinkLimitBps)
		current.DownlinkLimitBps = strictestPositiveUint64(current.DownlinkLimitBps, item.DownlinkLimitBps)
		current.MaxConnections = strictestPositiveUint64(current.MaxConnections, item.MaxConnections)
		byUsername[username] = current
	}
	out := make([]RuntimeUserLimitPolicy, 0, len(byUsername))
	for _, item := range byUsername {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out
}

func strictestPositiveUint64(current, candidate uint64) uint64 {
	if candidate == 0 {
		return current
	}
	if current == 0 || candidate < current {
		return candidate
	}
	return current
}

func loadRuntimeLimitPolicyValues(ctx context.Context, db *gorm.DB, fallbackUplink, fallbackDownlink uint64) (RuntimeLimitPolicyValues, error) {
	values := RuntimeLimitPolicyValues{
		UplinkLimitBps:   fallbackUplink,
		DownlinkLimitBps: fallbackDownlink,
	}
	if db == nil {
		return values, nil
	}
	rows := []model.Setting{}
	if err := db.WithContext(ctx).Where("key in ?", []string{
		runtimeInstanceUplinkLimitKey,
		runtimeInstanceDownlinkLimitKey,
		runtimeLimitPolicyVersionKey,
	}).Find(&rows).Error; err != nil {
		return values, err
	}
	for _, row := range rows {
		n, err := strconv.ParseUint(strings.TrimSpace(row.Value), 10, 64)
		if err != nil {
			return values, fmt.Errorf("invalid runtime limit setting %s: %w", row.Key, err)
		}
		switch row.Key {
		case runtimeInstanceUplinkLimitKey:
			values.UplinkLimitBps = n
		case runtimeInstanceDownlinkLimitKey:
			values.DownlinkLimitBps = n
		case runtimeLimitPolicyVersionKey:
			values.PolicyVersion = n
		}
	}
	return values, nil
}
