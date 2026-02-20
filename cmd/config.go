package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
)

const (
	runModeConsole = "console"
	runModeService = "service"
)

type Config struct {
	RunMode        string `json:"run_mode" long:"run-mode" choice:"console" choice:"service" env:"RUN_MODE" description:"Run mode: console or service"`
	ConfigPath     string `json:"config" long:"config" short:"c" env:"CONFIG" default:"" description:"Path to bot JSON config"`
	Token          string `json:"token" long:"token" short:"t" env:"TOKEN" default:"" description:"Telegram bot token"`
	XrayConfigsDir string `json:"xray_configs_dir" long:"xray-configs-dir" short:"d" env:"XRAY_CONFIGS_DIR" default:"" description:"Directory with Xray client configs"`
	XrayConfigPath string `json:"xray_config_path" long:"xray-config-path" short:"p" env:"XRAY_CONFIG_PATH" default:"" description:"Active Xray config path"`
	ServiceName    string `json:"service_name" long:"service-name" env:"SERVICE_NAME" default:"" description:"Systemd service name to restart"`
	LockTimeout    string `json:"lock_timeout" long:"lock-timeout" env:"LOCK_TIMEOUT" description:"Command lock timeout (e.g. 90s)"`
	LogLevel       string `json:"log_level" long:"log-level" env:"LOG_LEVEL" description:"Logger level: debug, info, warn, error"`
}

type bootstrapArgs struct {
	RunMode    string `long:"run-mode" choice:"console" choice:"service" env:"RUN_MODE"`
	ConfigPath string `long:"config" short:"c" env:"CONFIG" default:""`
}

type configOverrides struct {
	RunMode        *string `long:"run-mode" choice:"console" choice:"service" env:"RUN_MODE"`
	ConfigPath     *string `long:"config" short:"c" env:"CONFIG"`
	Token          *string `long:"token" short:"t" env:"TOKEN"`
	XrayConfigsDir *string `long:"xray-configs-dir" short:"d" env:"XRAY_CONFIGS_DIR"`
	XrayConfigPath *string `long:"xray-config-path" short:"p" env:"XRAY_CONFIG_PATH"`
	ServiceName    *string `long:"service-name" env:"SERVICE_NAME"`
	LockTimeout    *string `long:"lock-timeout" env:"LOCK_TIMEOUT"`
	LogLevel       *string `long:"log-level" env:"LOG_LEVEL"`
}

func LoadConfig(args []string) (Config, error) {
	bootstrap, err := parseBootstrapArgs(args)
	if err != nil {
		return Config{}, err
	}

	modeHint := bootstrap.RunMode
	if modeHint == "" {
		modeHint = runModeConsole
	}

	configPath := resolveConfigPath(modeHint, bootstrap.ConfigPath)
	configPathExplicit := configPathWasExplicit(args[1:])

	cfg, err := loadConfigFile(configPath, configPathExplicit, modeHint)
	if err != nil {
		return Config{}, err
	}

	overrides, err := parseOverrides(args)
	if err != nil {
		return Config{}, err
	}
	applyOverrides(&cfg, overrides)

	if cfg.ConfigPath == "" {
		cfg.ConfigPath = configPath
	}
	if cfg.RunMode == "" {
		cfg.RunMode = modeHint
	}

	applyCommonDefaults(&cfg)
	cfg, err = finalizeConfigByRunMode(cfg)
	if err != nil {
		return Config{}, err
	}

	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func parseBootstrapArgs(args []string) (bootstrapArgs, error) {
	bootstrap := bootstrapArgs{}
	parser := flags.NewParser(&bootstrap, flags.Default|flags.IgnoreUnknown)
	if _, err := parser.ParseArgs(args[1:]); err != nil {
		return bootstrapArgs{}, err
	}
	return bootstrap, nil
}

func loadConfigFile(configPath string, configPathExplicit bool, modeHint string) (Config, error) {
	cfg := Config{RunMode: modeHint}
	if configPath == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		if configPathExplicit {
			return Config{}, fmt.Errorf("config file not found: %s", configPath)
		}
		return cfg, nil
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config json: %w", err)
	}
	return cfg, nil
}

func parseOverrides(args []string) (configOverrides, error) {
	overrides := configOverrides{}
	parser := flags.NewParser(&overrides, flags.Default)
	if _, err := parser.ParseArgs(args[1:]); err != nil {
		return configOverrides{}, err
	}
	return overrides, nil
}

func applyOverrides(cfg *Config, overrides configOverrides) {
	if overrides.RunMode != nil {
		cfg.RunMode = *overrides.RunMode
	}
	if overrides.ConfigPath != nil {
		cfg.ConfigPath = *overrides.ConfigPath
	}
	if overrides.Token != nil {
		cfg.Token = *overrides.Token
	}
	if overrides.XrayConfigsDir != nil {
		cfg.XrayConfigsDir = *overrides.XrayConfigsDir
	}
	if overrides.XrayConfigPath != nil {
		cfg.XrayConfigPath = *overrides.XrayConfigPath
	}
	if overrides.ServiceName != nil {
		cfg.ServiceName = *overrides.ServiceName
	}
	if overrides.LockTimeout != nil {
		cfg.LockTimeout = *overrides.LockTimeout
	}
	if overrides.LogLevel != nil {
		cfg.LogLevel = *overrides.LogLevel
	}
}

func applyCommonDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.LockTimeout) == "" {
		cfg.LockTimeout = "90s"
	}
	if strings.TrimSpace(cfg.LogLevel) == "" {
		cfg.LogLevel = "info"
	}
}

func finalizeConfigByRunMode(cfg Config) (Config, error) {
	switch cfg.RunMode {
	case runModeConsole:
		return finalizeConsoleConfig(cfg), nil
	case runModeService:
		return finalizeServiceConfig(cfg), nil
	default:
		return Config{}, fmt.Errorf("unsupported run mode: %s", cfg.RunMode)
	}
}

func finalizeConsoleConfig(cfg Config) Config {
	if cfg.XrayConfigsDir == "" {
		cfg.XrayConfigsDir = "./xray-configs"
	}
	if cfg.XrayConfigPath == "" {
		cfg.XrayConfigPath = "./xray-configs/config.json"
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "xray"
	}
	return cfg
}

func finalizeServiceConfig(cfg Config) Config {
	if cfg.XrayConfigsDir == "" {
		cfg.XrayConfigsDir = "/usr/local/etc/xray"
	}
	if cfg.XrayConfigPath == "" {
		cfg.XrayConfigPath = "/etc/xray/config.json"
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "xray"
	}
	return cfg
}

func resolveConfigPath(runMode string, provided string) string {
	if strings.TrimSpace(provided) != "" {
		return provided
	}

	if envPath, ok := os.LookupEnv("CONFIG"); ok && strings.TrimSpace(envPath) != "" {
		return envPath
	}

	if runMode == runModeService {
		return "/etc/xray-tlg/config.json"
	}

	wd, err := os.Getwd()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(wd, "config.json")
}

func configPathWasExplicit(args []string) bool {
	if _, ok := os.LookupEnv("CONFIG"); ok {
		return true
	}
	return hasFlagArg(args, "-c", "--config")
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Token) == "" {
		return errors.New("token is required")
	}
	if strings.TrimSpace(cfg.XrayConfigsDir) == "" {
		return errors.New("xray configs dir is required")
	}
	if strings.TrimSpace(cfg.XrayConfigPath) == "" {
		return errors.New("xray config path is required")
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return errors.New("service name is required")
	}
	duration, err := time.ParseDuration(cfg.LockTimeout)
	if err != nil || duration <= 0 {
		return errors.New("lock timeout must be greater than zero")
	}
	return nil
}

func hasFlagArg(args []string, short string, long string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == short || arg == long {
			return true
		}
		if strings.HasPrefix(arg, short+"=") || strings.HasPrefix(arg, long+"=") {
			return true
		}
		if arg == "--" {
			return false
		}
	}
	return false
}
