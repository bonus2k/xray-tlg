APP_NAME := xray-tlg
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
INSTALL_BIN := /usr/local/bin/$(APP_NAME)
SYSTEMD_UNIT := /etc/systemd/system/$(APP_NAME).service
SERVICE_CONFIG_DIR := /etc/xray-tlg
SERVICE_ENV_FILE := $(SERVICE_CONFIG_DIR)/$(APP_NAME).env
SERVICE_CONFIG_FILE := $(SERVICE_CONFIG_DIR)/config.json

TOKEN ?=
XRAY_CONFIGS_DIR ?= /usr/local/etc/xray
XRAY_CONFIG_PATH ?= /etc/xray/config.json
XRAY_SERVICE_NAME ?= xray
LOCK_TIMEOUT ?= 90s
LOG_LEVEL ?= info
GOCACHE ?= /tmp/go-build

.PHONY: build test run run-local check-copy clean install install-service uninstall-service ensure-binary

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) ./cmd

test:
	GOCACHE=$(GOCACHE) go test ./...

run:
	go run ./cmd --run-mode=console

run-local:
	@if [ -z "$(TOKEN)" ]; then \
		echo "TOKEN is required. Usage: make run-local TOKEN=<telegram_token>"; \
		exit 1; \
	fi
	go run ./cmd --run-mode=console --config=./configs/config.local.example.json --token=$(TOKEN)

check-copy:
	GOCACHE=$(GOCACHE) go test ./internal/handlers -run TestCopyConfigFileFromTestdata -v

clean:
	rm -rf $(BIN_DIR)

ensure-binary:
	@test -x $(BIN_PATH) || (echo "Binary $(BIN_PATH) not found. Run: make build"; exit 1)

install: ensure-binary
	install -m 0755 $(BIN_PATH) $(INSTALL_BIN)

install-service: install
	@if [ -z "$(TOKEN)" ]; then \
		echo "TOKEN is required. Usage: make install-service TOKEN=<telegram_token>"; \
		exit 1; \
	fi
	install -m 0644 deploy/$(APP_NAME).service $(SYSTEMD_UNIT)
	mkdir -p $(SERVICE_CONFIG_DIR)
	printf 'TOKEN=%s\n' '$(TOKEN)' > $(SERVICE_ENV_FILE)
	chmod 600 $(SERVICE_ENV_FILE)
	printf '{\n  "run_mode": "service",\n  "token": "",\n  "xray_configs_dir": "%s",\n  "xray_config_path": "%s",\n  "service_name": "%s",\n  "lock_timeout": "%s",\n  "log_level": "%s"\n}\n' \
		'$(XRAY_CONFIGS_DIR)' \
		'$(XRAY_CONFIG_PATH)' \
		'$(XRAY_SERVICE_NAME)' \
		'$(LOCK_TIMEOUT)' \
		'$(LOG_LEVEL)' > $(SERVICE_CONFIG_FILE)
	chmod 640 $(SERVICE_CONFIG_FILE)
	systemctl daemon-reload
	systemctl enable --now $(APP_NAME)

uninstall-service:
	systemctl disable --now $(APP_NAME) || true
	rm -f $(SYSTEMD_UNIT)
	rm -f $(SERVICE_ENV_FILE)
	systemctl daemon-reload
	rm -f $(INSTALL_BIN)
