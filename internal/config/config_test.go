package config

import "testing"

func TestLoadDefaultsRuntimeCaptureDisabled(t *testing.T) {
	t.Setenv("XTOOL_RUNTIME_CAPTURE_ENABLED", "")

	cfg := Load()
	if cfg.RuntimeCaptureEnabled {
		t.Fatal("expected runtime capture to be disabled by default")
	}
}

func TestLoadEnablesRuntimeCaptureFromEnv(t *testing.T) {
	t.Setenv("XTOOL_RUNTIME_CAPTURE_ENABLED", "true")

	cfg := Load()
	if !cfg.RuntimeCaptureEnabled {
		t.Fatal("expected runtime capture to be enabled from env")
	}
}
