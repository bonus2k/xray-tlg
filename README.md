# xray-tlg

Telegram-бот для управления Xray:
- выбор и применение клиентского конфига
- запуск speedtest
- перезапуск systemd-сервиса Xray

## Возможности

- Логирование через `uber/zap` (консольный формат, уровни логов)
- Потокобезопасный `handler` с mutex-блокировкой команд
- Блокировка новых команд на время выполнения операции (по умолчанию 90 секунд)
- Единая обработка ошибок: пользователю показывается безопасное сообщение, полная ошибка уходит в лог
- Более информативные Telegram-меню и сообщения с emoji
- Форматированный отчёт speedtest (HTML)
- Тестовые данные для локальной проверки копирования конфигов

## Конфигурация

Поддерживаются режимы запуска:
- `console` — локальный запуск из консоли
- `service` — запуск как systemd-сервис

### Чем отличаются `run_mode`

- `console`:
  - предназначен для локального/ручного запуска;
  - дефолтный путь конфига: `./config.json` (в рабочей директории);
  - дефолтные пути Xray: `./xray-configs` и `./xray-configs/config.json`;
  - удобно для разработки и проверки на тестовых данных.
- `service`:
  - предназначен для запуска через `systemd`;
  - дефолтный путь конфига: `/etc/xray-tlg/config.json`;
  - дефолтные пути Xray: `/usr/local/etc/xray` и `/etc/xray/config.json`;
  - используется unit-файл `deploy/xray-tlg.service` и env-файл `/etc/xray-tlg/xray-tlg.env`.

Параметры можно задавать через:
- JSON-конфиг (`--config` / `CONFIG`)
- CLI-флаги
- переменные окружения

Примеры конфигов:
- `configs/config.console.example.json`
- `configs/config.local.example.json`
- `configs/config.service.example.json`

Тестовые данные:
- `testdata/xray-configs/*.json` — набор конфигов для выбора
- `testdata/active/config.json` — целевой active-файл

## Основные флаги

- `--run-mode=console|service`
- `--config=/path/to/config.json`
- `--token=<telegram_token>`
- `--xray-configs-dir=/path/to/xray-configs`
- `--xray-config-path=/path/to/active/config.json`
- `--service-name=xray`
- `--lock-timeout=90s`
- `--log-level=debug|info|warn|error`

## Локальный запуск

Запуск бота на локальных тестовых данных:

```bash
make run-local TOKEN=<telegram_token>
```

Эта команда использует `configs/config.local.example.json`, где:
- `xray_configs_dir=./testdata/xray-configs`
- `xray_config_path=./testdata/active/config.json`

Проверка копирования конфигов тестом:

```bash
make check-copy
```

## Сборка и тесты

```bash
make build
make test
```

## Установка как systemd-сервис

Рекомендуемый порядок (чтобы `sudo` не требовал `go` в `PATH`):

```bash
make build
sudo make install-service TOKEN=<telegram_token>
```

`install-service` ожидает, что бинарник уже собран (`bin/xray-tlg`).

Можно установить только бинарник:

```bash
make build
sudo make install
```

Требования:
- запуск с правами, достаточными для записи в `/usr/local/bin`, `/etc/systemd/system` и `/etc/xray-tlg`;
- установленный `systemd` (`systemctl`).

Дополнительные параметры установки (опционально):

```bash
sudo make install-service \
  TOKEN=<telegram_token> \
  XRAY_CONFIGS_DIR=/usr/local/etc/xray \
  XRAY_CONFIG_PATH=/etc/xray/config.json \
  XRAY_SERVICE_NAME=xray \
  LOCK_TIMEOUT=90s \
  LOG_LEVEL=info
```

Что делает цель:
- ставит бинарник в `/usr/local/bin/xray-tlg`
- ставит unit в `/etc/systemd/system/xray-tlg.service`
- создаёт `/etc/xray-tlg/xray-tlg.env` и записывает в него `TOKEN`
- создаёт `/etc/xray-tlg/config.json` с параметрами сервиса
- выполняет `systemctl daemon-reload`
- включает и запускает сервис

Unit-файл: `deploy/xray-tlg.service`

Проверка после установки:

```bash
systemctl status xray-tlg
journalctl -u xray-tlg -f
```

Обновление параметров сервиса:
- повторно запустить `make install-service ...` с новыми значениями;
- цель перезапишет env/config, выполнит `daemon-reload` и перезапустит сервис через `enable --now`.

## Удаление сервиса

```bash
make uninstall-service
```
