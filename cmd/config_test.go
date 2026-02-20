package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigConsoleDefaults(t *testing.T) {
	unsetEnv(t, "CONFIG")
	unsetEnv(t, "RUN_MODE")

	cfg, err := LoadConfig([]string{"xray-tlg", "--token=test-token"})
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.RunMode != runModeConsole {
		t.Fatalf("unexpected run mode: %s", cfg.RunMode)
	}
	if cfg.XrayConfigsDir != "./xray-configs" {
		t.Fatalf("unexpected xray configs dir: %s", cfg.XrayConfigsDir)
	}
	if cfg.XrayConfigPath != "./xray-configs/config.json" {
		t.Fatalf("unexpected xray config path: %s", cfg.XrayConfigPath)
	}
	if cfg.ServiceName != "xray" {
		t.Fatalf("unexpected service name: %s", cfg.ServiceName)
	}
	timeout, err := time.ParseDuration(cfg.LockTimeout)
	if timeout != 90*time.Second || err != nil {
		t.Fatalf("unexpected lock timeout: %s", cfg.LockTimeout)
	}
}

func TestLoadConfigServiceDefaults(t *testing.T) {
	unsetEnv(t, "CONFIG")
	unsetEnv(t, "RUN_MODE")

	cfg, err := LoadConfig([]string{"xray-tlg", "--run-mode=service", "--token=test-token"})
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.RunMode != runModeService {
		t.Fatalf("unexpected run mode: %s", cfg.RunMode)
	}
	if cfg.XrayConfigsDir != "/usr/local/etc/xray" {
		t.Fatalf("unexpected xray configs dir: %s", cfg.XrayConfigsDir)
	}
	if cfg.XrayConfigPath != "/etc/xray/config.json" {
		t.Fatalf("unexpected xray config path: %s", cfg.XrayConfigPath)
	}
	if cfg.ConfigPath != "/etc/xray-tlg/config.json" {
		t.Fatalf("unexpected config path: %s", cfg.ConfigPath)
	}
}

func TestResolveConfigPathConsole(t *testing.T) {
	path := resolveConfigPath(runModeConsole, "")
	if filepath.Base(path) != "config.json" {
		t.Fatalf("unexpected config file name: %s", path)
	}
}

func TestHasFlagArg(t *testing.T) {
	args := []string{"--token=abc", "--config=/tmp/config.json"}
	if !hasFlagArg(args, "-c", "--config") {
		t.Fatalf("expected hasFlagArg to find --config")
	}
}

func TestLoadConfigFromJSONDoesNotGetOverwrittenByDefaults(t *testing.T) {
	unsetEnv(t, "CONFIG")
	unsetEnv(t, "RUN_MODE")
	unsetEnv(t, "TOKEN")

	tempConfigFile, err := os.CreateTemp(t.TempDir(), "xray-tlg-*.json")
	if err != nil {
		t.Fatalf("create temp config failed: %v", err)
	}
	defer func() {
		_ = tempConfigFile.Close()
	}()

	content := `{
  "run_mode": "service",
  "token": "json-token",
  "xray_configs_dir": "/tmp/custom-configs",
  "xray_config_path": "/tmp/custom-active.json",
  "service_name": "custom-xray",
  "lock_timeout": "33s",
  "log_level": "debug"
}`
	if _, err := tempConfigFile.WriteString(content); err != nil {
		t.Fatalf("write temp config failed: %v", err)
	}

	cfg, err := LoadConfig([]string{"xray-tlg", "--config=" + tempConfigFile.Name()})
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.RunMode != runModeService {
		t.Fatalf("run_mode overridden unexpectedly: %s", cfg.RunMode)
	}
	if cfg.ServiceName != "custom-xray" {
		t.Fatalf("service_name overridden unexpectedly: %s", cfg.ServiceName)
	}
	if cfg.XrayConfigsDir != "/tmp/custom-configs" {
		t.Fatalf("xray_configs_dir overridden unexpectedly: %s", cfg.XrayConfigsDir)
	}
	if cfg.XrayConfigPath != "/tmp/custom-active.json" {
		t.Fatalf("xray_config_path overridden unexpectedly: %s", cfg.XrayConfigPath)
	}
	if cfg.LockTimeout != "33s" {
		t.Fatalf("lock_timeout overridden unexpectedly: %s", cfg.LockTimeout)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("log_level overridden unexpectedly: %s", cfg.LogLevel)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	value, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env %s failed: %v", key, err)
	}

	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, value)
			return
		}
		_ = os.Unsetenv(key)
	})
}
