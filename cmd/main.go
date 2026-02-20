package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"time"

	"github.com/bonus2k/xray-tlg/internal/handlers"
	"github.com/bonus2k/xray-tlg/internal/logger"
	"github.com/bonus2k/xray-tlg/internal/router"
	"github.com/go-telegram/bot"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

func main() {
	cfg, err := LoadConfig(os.Args)
	if err != nil {
		var fe *flags.Error
		if errors.As(err, &fe) && errors.Is(fe.Type, flags.ErrHelp) {
			os.Exit(0)
		}
		exitWithBootstrapError("config error", err)
	}

	appLogger, err := logger.New(cfg.LogLevel)
	if err != nil {
		exitWithBootstrapError("logger init error", err)
	}
	defer func() {
		_ = appLogger.Sync()
	}()
	duration, _ := time.ParseDuration(cfg.LockTimeout)
	appLogger.Info("configuration loaded",
		zap.String("run_mode", cfg.RunMode),
		zap.String("config_path", cfg.ConfigPath),
		zap.String("xray_configs_dir", cfg.XrayConfigsDir),
		zap.String("xray_config_path", cfg.XrayConfigPath),
		zap.String("service_name", cfg.ServiceName),
		zap.Duration("lock_timeout", duration),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	handler, err := handlers.NewHandler(cfg.XrayConfigsDir, cfg.XrayConfigPath, cfg.ServiceName, duration, appLogger)
	if err != nil {
		appLogger.Error("handler init failed", zap.Error(err))
		os.Exit(1)
	}

	opts := router.GetRouter(handler)

	telegramBot, err := bot.New(cfg.Token, opts...)
	if err != nil {
		appLogger.Error("telegram bot init failed", zap.Error(err))
		os.Exit(1)
	}

	appLogger.Info("bot started")
	telegramBot.Start(ctx)
	appLogger.Info("bot stopped")
}

func exitWithBootstrapError(message string, err error) {
	bootstrapLogger, loggerErr := logger.New("error")
	if loggerErr != nil {
		os.Exit(1)
	}
	defer func() {
		_ = bootstrapLogger.Sync()
	}()

	bootstrapLogger.Error(message, zap.Error(err))
	os.Exit(1)
}
