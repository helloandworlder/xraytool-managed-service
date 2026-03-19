package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr          string
	DataDir             string
	DBPath              string
	BackupDir           string
	JWTSecret           string
	DefaultAdminUser    string
	DefaultAdminPass    string
	DefaultInboundPort  int
	ManagedXrayEnabled  bool
	XrayBinaryPath      string
	XrayWorkDir         string
	XrayConfigPath      string
	XrayAPIServer       string
	XrayCommandTag      string
	SchedulerInterval   time.Duration
	BarkBaseURLFallback string
	GoSeaTelemetry      GoSeaTelemetryConfig
}

type GoSeaTelemetryConfig struct {
	Enabled         bool
	BaseURL         string
	NodeID          string
	Username        string
	Password        string
	IntervalSeconds int
}

func Load() Config {
	dataDir := getEnv("XTOOL_DATA_DIR", "./data")
	xrayDir := getEnv("XTOOL_XRAY_DIR", filepath.Join(dataDir, "xray"))

	return Config{
		ListenAddr:          getEnv("XTOOL_LISTEN", ":18080"),
		DataDir:             dataDir,
		DBPath:              getEnv("XTOOL_DB_PATH", filepath.Join(dataDir, "xraytool.db")),
		BackupDir:           getEnv("XTOOL_BACKUP_DIR", filepath.Join(dataDir, "backups")),
		JWTSecret:           getEnv("XTOOL_JWT_SECRET", "change-me-please"),
		DefaultAdminUser:    getEnv("XTOOL_ADMIN_USER", "admin"),
		DefaultAdminPass:    getEnv("XTOOL_ADMIN_PASS", "admin123456"),
		DefaultInboundPort:  getEnvInt("XTOOL_DEFAULT_PORT", 23457),
		ManagedXrayEnabled:  getEnvBool("XTOOL_MANAGED_XRAY", true),
		XrayBinaryPath:      getEnv("XTOOL_XRAY_BIN", filepath.Join(xrayDir, "xray")),
		XrayWorkDir:         xrayDir,
		XrayConfigPath:      getEnv("XTOOL_XRAY_CONFIG", filepath.Join(xrayDir, "config.json")),
		XrayAPIServer:       getEnv("XTOOL_XRAY_API", "127.0.0.1:10085"),
		XrayCommandTag:      "api",
		SchedulerInterval:   time.Duration(getEnvInt("XTOOL_SCHEDULER_SECONDS", 30)) * time.Second,
		BarkBaseURLFallback: getEnv("XTOOL_BARK_BASE_URL", ""),
		GoSeaTelemetry: GoSeaTelemetryConfig{
			Enabled:         getEnvBool("XTOOL_GOSEALIGHT_TELEMETRY_ENABLED", false),
			BaseURL:         getEnv("XTOOL_GOSEALIGHT_BASE_URL", ""),
			NodeID:          getEnv("XTOOL_GOSEALIGHT_NODE_ID", ""),
			Username:        getEnv("XTOOL_GOSEALIGHT_NODE_USERNAME", ""),
			Password:        getEnv("XTOOL_GOSEALIGHT_NODE_PASSWORD", ""),
			IntervalSeconds: getEnvInt("XTOOL_GOSEALIGHT_TELEMETRY_INTERVAL_SECONDS", 60),
		},
	}
}

func EnsurePaths(cfg Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.BackupDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.XrayWorkDir, 0o755); err != nil {
		return err
	}
	return nil
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
