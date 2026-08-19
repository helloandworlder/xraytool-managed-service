package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"xraytool/internal/buildinfo"
	"xraytool/internal/config"
	"xraytool/internal/model"
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
	NodeID             string                     `json:"nodeId"`
	Username           string                     `json:"username"`
	Password           string                     `json:"password"`
	ProtocolVersion    string                     `json:"protocolVersion,omitempty"`
	Version            string                     `json:"version,omitempty"`
	Capabilities       []string                   `json:"capabilities,omitempty"`
	LimitPolicy        goSeaTelemetryLimitPolicy  `json:"limitPolicy,omitempty"`
	XrayCore           goSeaTelemetryXrayCore     `json:"xrayCore,omitempty"`
	LifecycleStatus    string                     `json:"lifecycleStatus,omitempty"`
	LifecycleMessage   string                     `json:"lifecycleMessage,omitempty"`
	LifecycleUpdatedAt string                     `json:"lifecycleUpdatedAt,omitempty"`
	SnapshotAt         string                     `json:"snapshotAt,omitempty"`
	Stats              goSeaTelemetryPayloadStats `json:"stats"`
}

type goSeaTelemetryLimitPolicy struct {
	AccountHardLimit        bool     `json:"accountHardLimit"`
	InstanceDirectional     bool     `json:"instanceDirectional"`
	Fields                  []string `json:"fields"`
	TargetUplinkLimitBPS    uint64   `json:"targetUplinkLimitBps"`
	TargetDownlinkLimitBPS  uint64   `json:"targetDownlinkLimitBps"`
	AppliedUplinkLimitBPS   uint64   `json:"appliedUplinkLimitBps"`
	AppliedDownlinkLimitBPS uint64   `json:"appliedDownlinkLimitBps"`
	PolicyVersion           uint64   `json:"policyVersion"`
	AppliedAt               string   `json:"appliedAt,omitempty"`
	LastApplyError          string   `json:"lastApplyError,omitempty"`
}

type goSeaTelemetryXrayCore struct {
	BinaryPath string `json:"binaryPath,omitempty"`
	Version    string `json:"version,omitempty"`
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
	lifecycle, err := s.store.GetNodeLifecycle()
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
		LimitPolicy:     s.runtimeLimitPolicyTelemetry(),
		XrayCore: goSeaTelemetryXrayCore{
			BinaryPath: s.cfg.XrayBinaryPath,
			Version:    detectXrayCoreVersion(s.cfg.XrayBinaryPath),
		},
		LifecycleStatus:    lifecycle.Status,
		LifecycleMessage:   lifecycle.Message,
		LifecycleUpdatedAt: lifecycle.UpdatedAt,
		SnapshotAt:         snapshot.UpdatedAt.UTC().Format(time.RFC3339Nano),
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

func (s *GoSeaLightTelemetryService) runtimeLimitPolicyTelemetry() goSeaTelemetryLimitPolicy {
	state := goSeaTelemetryLimitPolicy{
		AccountHardLimit:    true,
		InstanceDirectional: true,
		Fields: []string{
			"uplinkLimitBps",
			"downlinkLimitBps",
			"maxConnections",
		},
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		s.logger.Warn("load runtime limit telemetry settings failed", zap.Error(err))
		state.LastApplyError = err.Error()
		return state
	}
	state.TargetUplinkLimitBPS = normalizeRuntimeLimitBps(parseTelemetryUint(settings["runtime_instance_uplink_limit_bps"]))
	state.TargetDownlinkLimitBPS = normalizeRuntimeLimitBps(parseTelemetryUint(settings["runtime_instance_downlink_limit_bps"]))
	state.PolicyVersion = parseTelemetryUint(settings["runtime_limit_policy_version"])

	var latest model.RuntimeSyncTask
	if err := s.store.DB().Where("reason = ?", "limit_policy_reapply").Order("id desc").First(&latest).Error; err == nil {
		if latest.Error != "" {
			state.LastApplyError = latest.Error
		}
	}
	var applied model.RuntimeSyncTask
	if err := s.store.DB().Where("reason = ? and status = ?", "limit_policy_reapply", model.RuntimeSyncStatusSuccess).Order("id desc").First(&applied).Error; err == nil {
		state.AppliedUplinkLimitBPS = applied.DesiredUplinkLimitBps
		state.AppliedDownlinkLimitBPS = applied.DesiredDownlinkLimitBps
		if applied.AppliedAt != nil {
			state.AppliedAt = applied.AppliedAt.UTC().Format(time.RFC3339Nano)
		}
	}
	return state
}

func parseTelemetryUint(raw string) uint64 {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func detectXrayCoreVersion(binaryPath string) string {
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binaryPath, "version").CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
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
