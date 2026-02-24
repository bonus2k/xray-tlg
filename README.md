# xray-tlg

[![CI](https://github.com/bonus2k/xray-tlg/actions/workflows/action1.yml/badge.svg)](https://github.com/bonus2k/xray-tlg/actions/workflows/action1.yml)
[![Go Version](https://img.shields.io/badge/go-1.25.6-00ADD8.svg)](https://go.dev/)

A Telegram bot for managing Xray/V2Ray client configs on Linux servers: apply config files, run speed tests, and restart the Xray systemd service.

Xray Telegram bot, Xray config switcher, V2Ray Telegram automation, Linux network bot, systemd service restart bot.

## Table of Contents

- [Features](#features)
- [Use Cases](#use-cases)
- [How It Works](#how-it-works)
- [Requirements](#requirements)
- [Quick Start (Local)](#quick-start-local)
- [Run Modes](#run-modes)
- [Configuration](#configuration)
- [CLI Flags](#cli-flags)
- [Build, Test, Lint](#build-test-lint)
- [Systemd Installation](#systemd-installation)
- [Uninstall Service](#uninstall-service)
- [Project Structure](#project-structure)
- [Contributing](#contributing)

## Features

- Switch active Xray client config from Telegram inline menus.
- Run speedtest and return formatted HTML results in chat.
- Restart a target systemd service (default: `xray`).

## Use Cases

- Self-hosted VPN/proxy maintenance from mobile Telegram.
- Fast rollback between multiple Xray client profiles.
- Quick network diagnostics (latency/jitter/download/upload) without SSH.
- Lightweight control plane for single-node Xray setups.

## How It Works

1. Prepare multiple Xray client config files in `xray_configs_dir` (for example, one file per VPN location/provider).
2. The bot reads that directory and shows the available config files in a Telegram inline menu.
3. When you select a file, the bot copies it to the active config path (`xray_config_path`) as the live `config.json`.
4. The bot can then restart the target service (`xray` by default), so Xray loads the new active config.
5. A lock is enabled during execution (`lock_timeout`) to prevent concurrent actions.
6. Result messages are sent to the user, while technical details and errors are written to logs.

## Requirements

- Go `1.25.6`
- Linux (for `service` mode you need `systemd`)
- Telegram Bot Token
- Read access to config directory and write access to active config path

## Quick Start (Local)

Run using local test data from `testdata/`:

```bash
make run-local TOKEN=<telegram_token>
```

This command uses `configs/config.local.example.json`:

- `xray_configs_dir=./testdata/xray-configs`
- `xray_config_path=./testdata/active/config.json`

Validate config copy behavior:

```bash
make check-copy
```

## Run Modes

Two modes are supported:

- `console`:
  - manual/local run;
  - default config path: `./config.json`;
  - default Xray paths: `./xray-configs` and `./xray-configs/config.json`.
- `service`:
  - run as systemd service;
  - default config path: `/etc/xray-tlg/config.json`;
  - default Xray paths: `/usr/local/etc/xray` and `/etc/xray/config.json`.

## Configuration

Configuration precedence:

1. CLI flags
2. Environment variables
3. JSON config file
4. Built-in defaults

Example configs:

- `configs/config.console.example.json`
- `configs/config.local.example.json`
- `configs/config.service.example.json`

Minimal config example:

```json
{
  "run_mode": "console",
  "token": "123456:telegram-bot-token",
  "xray_configs_dir": "./testdata/xray-configs",
  "xray_config_path": "./testdata/active/config.json",
  "service_name": "xray",
  "lock_timeout": "90s",
  "log_level": "info"
}
```

## CLI Flags

```text
--run-mode=console|service
--config=/path/to/config.json
--token=<telegram_token>
--xray-configs-dir=/path/to/xray-configs
--xray-config-path=/path/to/active/config.json
--service-name=xray
--lock-timeout=90s
--log-level=debug|info|warn|error
```

## Build, Test, Lint

```bash
make build
make test
```

Additional checks (same as CI policy):

```bash
go vet ./...
gofmt -s -w .
```

## Systemd Installation

Recommended flow:

```bash
make build
sudo make install-service TOKEN=<telegram_token>
```

`install-service` will:

- Install binary to `/usr/local/bin/xray-tlg`.
- Install unit file to `/etc/systemd/system/xray-tlg.service`.
- Create `/etc/xray-tlg/xray-tlg.env` and write `TOKEN`.
- Create `/etc/xray-tlg/config.json` for `run_mode=service`.
- Run `systemctl daemon-reload` and `systemctl enable --now xray-tlg`.

Optional installation parameters:

```bash
sudo make install-service \
  TOKEN=<telegram_token> \
  XRAY_CONFIGS_DIR=/usr/local/etc/xray \
  XRAY_CONFIG_PATH=/etc/xray/config.json \
  XRAY_SERVICE_NAME=xray \
  LOCK_TIMEOUT=90s \
  LOG_LEVEL=info
```

Check service state:

```bash
systemctl status xray-tlg
journalctl -u xray-tlg -f
```

Unit file path: `deploy/xray-tlg.service`

## Uninstall Service

```bash
make uninstall-service
```

## Project Structure

```text
.
├── cmd/                 # entrypoint and config loading
├── internal/
│   ├── handlers/        # bot command logic
│   ├── logger/          # zap logger setup
│   └── router/          # telegram handler routing
├── configs/             # example configs
├── deploy/              # systemd unit
├── testdata/            # test xray configs
├── .github/workflows/   # CI
└── Makefile
```

## Contributing

1. Create a branch: `git checkout -b feature/my-change`
2. Run checks: `make test`, `go vet ./...`, `gofmt -s -w .`
3. Open a Pull Request with a clear description.

If your change affects command behavior or config format, add/update tests in `cmd` or `internal/handlers`.

