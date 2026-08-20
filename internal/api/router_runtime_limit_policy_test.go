package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"xraytool/internal/auth"
	"xraytool/internal/config"
	"xraytool/internal/db"
	"xraytool/internal/service"
	"xraytool/internal/store"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRuntimeLimitPolicyHTTPQueuesAndExposesTask(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "xraytool.db"))
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	st := store.New(database)
	require.NoError(t, st.EnsureDefaultSettings(23457, "", nil))
	orders := service.NewOrderService(database, nil, zap.NewNop())
	api := New(
		database,
		st,
		orders,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		config.Config{JWTSecret: "test-secret"},
		zap.NewNop(),
	)
	token, err := auth.GenerateToken("test-secret", 1, "admin", time.Hour)
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"uplinkLimitBps":   15_000_000,
		"downlinkLimitBps": 12_000_000,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/limit-policy/reapply", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	queued := httptest.NewRecorder()
	api.Router().ServeHTTP(queued, req)
	require.Equal(t, http.StatusAccepted, queued.Code)

	var queuedPayload struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
		Task   struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal(queued.Body.Bytes(), &queuedPayload))
	require.True(t, queuedPayload.OK)
	require.Equal(t, "queued", queuedPayload.Status)
	require.Equal(t, "pending", queuedPayload.Task.Status)

	getReq := httptest.NewRequest(http.MethodGet, "/api/runtime/sync-tasks/"+strconv.FormatUint(uint64(queuedPayload.Task.ID), 10), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	got := httptest.NewRecorder()
	api.Router().ServeHTTP(got, getReq)
	require.Equal(t, http.StatusOK, got.Code)
}
