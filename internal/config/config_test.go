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

func TestLoadUses30MbpsDefaultsForMissingInstanceLimits(t *testing.T) {
	for _, value := range []string{"", "0"} {
		t.Setenv("XTOOL_INSTANCE_UPLINK_LIMIT_BPS", value)
		t.Setenv("XTOOL_INSTANCE_DOWNLINK_LIMIT_BPS", value)

		cfg := Load()
		if cfg.InstanceUplinkLimitBps != DefaultLimitBps {
			t.Fatalf("uplink value %q = %d, want %d", value, cfg.InstanceUplinkLimitBps, DefaultLimitBps)
		}
		if cfg.InstanceDownlinkLimitBps != DefaultLimitBps {
			t.Fatalf("downlink value %q = %d, want %d", value, cfg.InstanceDownlinkLimitBps, DefaultLimitBps)
		}
	}
}
