package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"xraytool/internal/buildinfo"
	"xraytool/internal/config"
	"xraytool/internal/store"

	"go.uber.org/zap"
)

type GoSeaLightTelemetryService struct {
	store   *store.Store
	runtime *RuntimeStatsService
	logger  *zap.Logger
	client  *http.Client
	cfg     config.GoSeaTelemetryConfig

	mu              sync.Mutex
	lastAttemptAt   time.Time
	peakUploadBPS   float64
	peakDownloadBPS float64
}

type goSeaTelemetrySettings struct {
	Enabled  bool
	BaseURL  string
	NodeID   string
	Username string
	Password string
	Interval time.Duration
}

type goSeaTelemetryPayload struct {
	NodeID          string                     `json:"nodeId"`
	Username        string                     `json:"username"`
	Password        string                     `json:"password"`
	ProtocolVersion string                     `json:"protocolVersion,omitempty"`
	Version         string                     `json:"version,omitempty"`
	Capabilities    []string                   `json:"capabilities,omitempty"`
	SnapshotAt      string                     `json:"snapshotAt,omitempty"`
	Stats           goSeaTelemetryPayloadStats `json:"stats"`
}

type goSeaTelemetryPayloadStats struct {
	TrafficTotalBytes int64                        `json:"trafficTotalBytes,omitempty"`
	Traffic24HBytes   int64                        `json:"traffic24hBytes,omitempty"`
	UploadBPS         float64                      `json:"uploadBps,omitempty"`
	DownloadBPS       float64                      `json:"downloadBps,omitempty"`
	PeakUploadBPS     float64                      `json:"peakUploadBps,omitempty"`
	PeakDownloadBPS   float64                      `json:"peakDownloadBps,omitempty"`
	ActiveConnections int64                        `json:"activeConnections,omitempty"`
	ActiveUsers       int64                        `json:"activeUsers,omitempty"`
	CPUPercent        float64                      `json:"cpuPercent,omitempty"`
	MemoryBytes       uint64                       `json:"memoryBytes,omitempty"`
	Routes            []goSeaTelemetryPayloadRoute `json:"routes,omitempty"`
}

type goSeaTelemetryPayloadRoute struct {
	Key               string  `json:"key"`
	Label             string  `json:"label,omitempty"`
	ActiveConnections int64   `json:"activeConnections,omitempty"`
	UploadBPS         float64 `json:"uploadBps,omitempty"`
	DownloadBPS       float64 `json:"downloadBps,omitempty"`
	Traffic24HBytes   int64   `json:"traffic24hBytes,omitempty"`
}

func NewGoSeaLightTelemetryService(st *store.Store, runtime *RuntimeStatsService, cfg config.GoSeaTelemetryConfig, logger *zap.Logger) *GoSeaLightTelemetryService {
	return &GoSeaLightTelemetryService{
		store:   st,
		runtime: runtime,
		logger:  logger,
		cfg:     cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *GoSeaLightTelemetryService) RunDue(ctx context.Context) {
	settings, err := s.loadSettings()
	if err != nil {
		s.logger.Warn("load gosealight telemetry settings failed", zap.Error(err))
		return
	}
	if !settings.Enabled {
		return
	}

	s.mu.Lock()
	if !s.lastAttemptAt.IsZero() && time.Since(s.lastAttemptAt) < settings.Interval {
		s.mu.Unlock()
		return
	}
	s.lastAttemptAt = time.Now()
	s.mu.Unlock()

	if err := s.push(ctx, settings); err != nil {
		s.logger.Warn("push gosealight telemetry failed", zap.Error(err))
		s.store.AddTaskLog("warn", "gosealight telemetry push failed", err.Error())
	}
}

func (s *GoSeaLightTelemetryService) loadSettings() (goSeaTelemetrySettings, error) {
	values, err := s.store.GetSettings()
	if err != nil {
		return goSeaTelemetrySettings{}, err
	}

	enabled := parseSettingBool(values["gosealight_telemetry_enabled"], s.cfg.Enabled)
	baseURL := firstNonEmptyTelemetry(values["gosealight_base_url"], s.cfg.BaseURL)
	nodeID := firstNonEmptyTelemetry(values["gosealight_node_id"], s.cfg.NodeID)
	username := firstNonEmptyTelemetry(values["gosealight_node_username"], s.cfg.Username)
	password := firstNonEmptyTelemetry(values["gosealight_node_password"], s.cfg.Password)
	intervalSeconds := parseSettingInt(values["gosealight_telemetry_interval_seconds"], s.cfg.IntervalSeconds)
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	return goSeaTelemetrySettings{
		Enabled:  enabled,
		BaseURL:  strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		NodeID:   strings.TrimSpace(nodeID),
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
		Interval: time.Duration(intervalSeconds) * time.Second,
	}, nil
}

func (s *GoSeaLightTelemetryService) push(ctx context.Context, settings goSeaTelemetrySettings) error {
	if settings.BaseURL == "" || settings.NodeID == "" || settings.Username == "" || settings.Password == "" {
		return fmt.Errorf("gosealight telemetry is enabled but configuration is incomplete")
	}

	snapshot, err := s.runtime.TelemetrySnapshot(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if snapshot.UploadBPS > s.peakUploadBPS {
		s.peakUploadBPS = snapshot.UploadBPS
	}
	if snapshot.DownloadBPS > s.peakDownloadBPS {
		s.peakDownloadBPS = snapshot.DownloadBPS
	}
	peakUpload := s.peakUploadBPS
	peakDownload := s.peakDownloadBPS
	s.mu.Unlock()

	routes := make([]goSeaTelemetryPayloadRoute, 0, len(snapshot.Routes))
	for _, route := range snapshot.Routes {
		routes = append(routes, goSeaTelemetryPayloadRoute{
			Key:               route.Key,
			Label:             route.Label,
			ActiveConnections: route.ActiveConnections,
			UploadBPS:         route.UploadBPS,
			DownloadBPS:       route.DownloadBPS,
			Traffic24HBytes:   route.Traffic24H,
		})
	}

	payload := goSeaTelemetryPayload{
		NodeID:          settings.NodeID,
		Username:        settings.Username,
		Password:        settings.Password,
		ProtocolVersion: buildinfo.ProtocolVersion,
		Version:         buildinfo.Version,
		Capabilities:    buildinfo.Capabilities(),
		SnapshotAt:      snapshot.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Stats: goSeaTelemetryPayloadStats{
			TrafficTotalBytes: snapshot.TrafficTotal,
			Traffic24HBytes:   snapshot.Traffic24H,
			UploadBPS:         snapshot.UploadBPS,
			DownloadBPS:       snapshot.DownloadBPS,
			PeakUploadBPS:     peakUpload,
			PeakDownloadBPS:   peakDownload,
			ActiveConnections: snapshot.ActiveConnections,
			ActiveUsers:       snapshot.ActiveUsers,
			CPUPercent:        derefFloat64(snapshot.CPUPercent),
			MemoryBytes:       snapshot.MemoryBytes,
			Routes:            routes,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.BaseURL+"/api/nodes/telemetry/ingest", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("telemetry ingest returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func firstNonEmptyTelemetry(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseSettingBool(value string, fallback bool) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return fallback
	}
	switch trimmed {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return fallback
	}
}

func parseSettingInt(value string, fallback int) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}
	return parsed
}

func derefFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
