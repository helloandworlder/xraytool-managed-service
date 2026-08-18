package service

import (
	"path/filepath"
	"testing"
	"time"

	"xraytool/internal/db"
	"xraytool/internal/model"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestReapplyLimitPolicyRuntimePersistsDirectionalPolicyAndStrictestUsers(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "xraytool.db"))
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	customer := model.Customer{Name: "limit-test-customer", Code: "limit-test"}
	require.NoError(t, database.Create(&customer).Error)
	order := model.Order{
		CustomerID: customer.ID,
		Name:       "limit-test",
		Mode:       model.OrderModeAuto,
		Status:     model.OrderStatusActive,
		Quantity:   1,
		Port:       1080,
		StartsAt:   time.Now().Add(-time.Hour),
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	require.NoError(t, database.Create(&order).Error)
	require.NoError(t, database.Create(&model.OrderItem{
		OrderID:  order.ID,
		IP:       "127.0.0.1",
		Port:     1080,
		Username: "alice",
		Password: "secret",
		Status:   model.OrderItemStatusActive,
	}).Error)

	service := NewOrderService(database, nil, zap.NewNop())
	first, err := service.ReapplyLimitPolicyRuntime(t.Context(), RuntimeLimitPolicyInput{
		UplinkLimitBps:   uint64Ptr(15_000_000),
		DownlinkLimitBps: uint64Ptr(12_000_000),
		Users: []RuntimeUserLimitPolicy{
			{Username: "alice", UplinkLimitBps: 10_000_000, DownlinkLimitBps: 8_000_000, MaxConnections: 10},
			{Username: " alice ", UplinkLimitBps: 5_000_000, DownlinkLimitBps: 0, MaxConnections: 4},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), first.PolicyVersion)
	require.Equal(t, uint64(15_000_000), first.DesiredUplinkLimitBps)
	require.Equal(t, uint64(12_000_000), first.DesiredDownlinkLimitBps)

	var item model.OrderItem
	require.NoError(t, database.First(&item, "username = ?", "alice").Error)
	require.Equal(t, int64(5_000_000), item.UplinkLimitBps)
	require.Equal(t, int64(8_000_000), item.DownlinkLimitBps)
	require.Equal(t, int64(4), item.MaxConnections)

	second, err := service.ReapplyLimitPolicyRuntime(t.Context(), RuntimeLimitPolicyInput{
		DownlinkLimitBps: uint64Ptr(9_000_000),
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), second.PolicyVersion)
	require.Equal(t, uint64(15_000_000), second.DesiredUplinkLimitBps)
	require.Equal(t, uint64(9_000_000), second.DesiredDownlinkLimitBps)

	var tasks []model.RuntimeSyncTask
	require.NoError(t, database.Order("id asc").Find(&tasks).Error)
	require.Len(t, tasks, 2)
}

func uint64Ptr(value uint64) *uint64 {
	return &value
}
