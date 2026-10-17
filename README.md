<div align="center">
  <img src="docs/images/logo.png" alt="DevicePulse Logo" width="180" style="border-radius: 24px; box-shadow: 0 8px 24px rgba(0,0,0,0.5);" />
  <h1>DevicePulse</h1>
  <p><b>Высокопроизводительная backend-платформа сбора, потоковой обработки и мониторинга телеметрии устройств</b></p>

  <p>
    <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go" /></a>
    <a href="https://www.postgresql.org/"><img src="https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white" alt="PostgreSQL" /></a>
    <a href="https://redis.io/"><img src="https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white" alt="Redis" /></a>
    <a href="https://nats.io/"><img src="https://img.shields.io/badge/NATS-JetStream-27AAE1?logo=natsdotio&logoColor=white" alt="NATS JetStream" /></a>
    <a href="https://grpc.io/"><img src="https://img.shields.io/badge/gRPC-v1.84-244c5a?logo=grpc&logoColor=white" alt="gRPC" /></a>
    <a href="https://www.docker.com/"><img src="https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white" alt="Docker" /></a>
    <img src="https://img.shields.io/badge/License-MIT-green" alt="License" />
  </p>
</div>

---

## 📌 О проекте

**DevicePulse** — комплексная серверная платформа промышленного уровня для приёма, потоковой обработки, долговременного хранения и анализа телеметрии, поступающей от IoT-устройств, микроконтроллеров и программных агентов.

Система спроектирована по канонам **Чистой Архитектуры (Clean / Hexagonal Architecture)** с абсолютным разделением доменной логики от деталей инфраструктуры, поддерживает двойной транспорт (высокоскоростной **REST API** и бинарный потоковый **gRPC**), асинхронный отказоустойчивый контур доставки событий на **NATS JetStream**, оперативное in-memory состояние в **Redis** и аналитическое хранилище временных рядов в **PostgreSQL 16**.

### Ключевые возможности:
- 🔐 **Безопасная аутентификация устройств**: Регистрация, генерация криптостойких API-ключей, хранение в виде SHA-256 хешей, отзыв и перевыпуск без перезапуска сервисов.
- ⚡ **Двойной транспорт приёма**: Приём одиночных и пакетных измерений через HTTP REST и потоковая передача через gRPC с защитой от перегрузки (Rate Limiting).
- 🔄 **Асинхронная потоковая обработка**: Гарантия доставки *At-Least-Once*, дедупликация событий на базе скользящих окон в Redis, обработка ошибок с `NakWithDelay` и изоляция сбойных пакетов в Dead Letter Queue (`telemetry.dlq`).
- ⏱️ **Детекция доступности (Heartbeat)**: Определение состояний устройств (`ONLINE`, `OFFLINE`, `UNKNOWN`) на базе времени последней активности с фоновым сканером таймаутов.
- 🚨 **Интеллектуальный алертинг**: Оценка пороговых правил (`>`, `<`, `>=`, `<=`, `==`), подавление дублирующих оповещений для одного инцидента, авто-закрытие при нормализации показателей и доставка уведомлений через Webhook с повторными попытками.
- 📊 **Оконная аналитика**: Расчет минимальных, максимальных и средних значений по минутным и часовым интервалам с использованием встроенной функции PostgreSQL `date_bin`.
- 🖥️ **Встроенный Web Dashboard**: Современный интерактивный веб-интерфейс на чистом HTML5/CSS/JS с графиками Chart.js, упакованный прямо в бинарник через `embed.FS` (работает сразу без Node.js и npm).

---

## 📸 Демонстрационный интерфейс (Web Dashboard)

Платформа оснащена встроенным дашбордом, доступным сразу при старте приложения по адресу `http://localhost:8080/`.

### 1. Управление устройствами и реестр подключений
Регистрация устройств, моментальный выпуск ключей с безопасным копированием, статистика доступности (`ONLINE`, `OFFLINE`, `UNKNOWN`).
<p align="center">
  <img src="docs/images/01-devices.png" alt="Реестр устройств" width="95%" />
</p>

### 2. Интерактивный эмулятор телеметрии и журнал событий
Ручная отправка одиночных измерений, быстрые пресеты для тестирования аномалий (перегрев) и запуск авто-потока с частотой 1 сек.
<p align="center">
  <img src="docs/images/02-simulator.png" alt="Эмулятор телеметрии" width="95%" />
</p>

### 3. Графики временных рядов в реальном времени
Карточки последних оперативных данных из Redis и динамические графики Chart.js с обновлением каждые 2 секунды.
<p align="center">
  <img src="docs/images/03-live-charts.png" alt="Графики реального времени" width="95%" />
</p>

### 4. Мониторинг инцидентов и конструктор правил оповещений
Настройка порогов, живая таблица инцидентов со статусами (`FIRING`, `ACKNOWLEDGED`, `RESOLVED`), подтверждение оператором и авто-закрытие при нормализации.
<p align="center">
  <img src="docs/images/04-alerts.png" alt="Инциденты и правила" width="95%" />
</p>

### 5. Расчет оконных агрегатов и аналитика
Группировка и расчет агрегированных метрик (Min, Max, Avg, Count) по временным окнам с вычислениями в PostgreSQL.
<p align="center">
  <img src="docs/images/05-analytics.png" alt="Аналитика и агрегаты" width="95%" />
</p>

---

## 🏗️ Структура репозитория

```
DevicePulse/
├── .github/
│   └── workflows/
│       └── ci.yml                 # Автоматизированный пайплайн сборки и тестов GitHub Actions
├── api/
│   ├── openapi/
│   │   └── openapi.yaml           # Полная спецификация REST API (OpenAPI 3.0)
│   └── proto/
│       └── telemetry.proto        # Protobuf контракты gRPC сервиса телеметрии
├── cmd/
│   ├── all-in-one/                # Единый раннер всех сервисов в одном процессе
│   ├── control-api/               # REST API управления конфигурацией, устройствами и алертами
│   ├── ingest-api/                # Высокопроизводительный приём телеметрии (REST + gRPC)
│   ├── telemetry-processor/       # Асинхронный обработчик очереди NATS, запись в БД и алертинг
│   ├── alert-worker/              # Воркер надёжной доставки Webhook-уведомлений
│   └── load-generator/            # Профессиональный генератор нагрузки (REST/gRPC, RPS, p95/p99)
├── docs/
│   └── images/                    # Логотип и скриншоты демонстрационного интерфейса
├── internal/
│   ├── app/                       # Точка сборки и запуска приложения
│   ├── config/                    # Загрузка и валидация конфигурации из окружения
│   ├── domain/                    # Чистые сущности предметной области и интерфейсы репозиториев
│   ├── queue/                     # Клиент NATS JetStream, публикация и работа с DLQ
│   ├── repository/
│   │   ├── postgres/              # Реализация репозиториев на pgx/v5 с пулом pgxpool и SQL
│   │   └── redis/                 # Хранилище оперативного состояния, дедупликация и rate limiting
│   ├── service/                   # Бизнес-логика (аутентификация, инжест, воркеры, алерты)
│   ├── telemetry/                 # Экспорт метрик Prometheus и pprof
│   └── transport/
│       ├── grpc/                  # Реализация gRPC сервера и сгенерированный код protobuf
│       └── http/                  # HTTP роутер Go 1.22+, middleware, хэндлеры и встроенный UI
├── migrations/
│   └── 000001_init.sql            # Схема базы данных с индексами и ограничениями целостности
├── docker-compose.yml             # Полный стек: PostgreSQL, Redis, NATS и приложение
├── Dockerfile                     # Multi-stage production сборка на базе Alpine
├── go.mod                         # Зависимости Go
├── LICENSE                        # Лицензия MIT
└── README.md                      # Документация проекта
```

---

## 🏛️ Архитектура и поток данных

### Архитектурная диаграмма компонентов

```mermaid
flowchart TD
    subgraph Clients["Источники данных и клиенты"]
        A[IoT Устройства / Агенты]
        B[Оператор / Панель мониторинга Web UI]
        C[Внешние системы / Webhook Receiver]
    end

    subgraph TransportLayer["Слой приёма (Ingest & Control)"]
        D[REST Ingest API]
        E[gRPC Ingest Server]
        F[Control REST API & Web Dashboard]
    end

    subgraph MessageBroker["Очередь сообщений (NATS JetStream)"]
        G[(telemetry.raw)]
        H[(telemetry.alerts)]
        I[(telemetry.dlq)]
    end

    subgraph AsyncProcessing["Асинхронные обработчики"]
        J[Telemetry Processor Worker Pool]
        K[Alert & Webhook Worker]
        L[Heartbeat / Offline Detector]
    end

    subgraph Storage["Хранилища данных"]
        M[(PostgreSQL 16\nУстройства, Метрики, Измерения,\nПравила, Инциденты, Доставки)]
        N[(Redis 7\nАктуальное состояние, Дедупликация,\nRate Limiting)]
    end

    A -->|HTTP POST| D
    A -->|gRPC Batch/Stream| E
    B -->|REST & Web UI| F

    D -->|Publish| G
    E -->|Publish| G

    D <-->|Rate Limit & Auth| N
    E <-->|Rate Limit & Auth| N

    G -->|Pull Consumer| J
    J -->|Dedup Check & State Update| N
    J -->|Batch Insert| M
    J -->|Threshold Breached / Resolved| H
    J -->|Corrupt / Max Deliveries Exceeded| I

    H -->|Pull Consumer| K
    K -->|HTTP POST with Backoff| C
    K -->|Record Delivery Status| M

    L -->|Periodic Scan| M
    L -->|Mark OFFLINE| N

    F -->|Query / Mutate| M
    F -->|Get Latest State| N
```

### Диаграмма последовательности обработки измерений

```mermaid
sequenceDiagram
    autonumber
    participant D as Устройство / Агент
    participant I as Ingest API (REST/gRPC)
    participant R as Redis
    participant Q as NATS JetStream
    participant P as Telemetry Processor
    participant DB as PostgreSQL
    participant W as Alert Worker
    participant WH as Внешний Webhook

    D->>I: Отправка пакета телеметрии (X-Device-Key)
    I->>DB: Проверка хеша ключа и активности устройства
    I->>R: Проверка Rate Limit (Incr window key)
    alt Превышен лимит или невалидный ключ
        I-->>D: 401 Unauthorized / 429 Too Many Requests
    end
    I->>I: Валидация меток времени и диапазонов значений
    I->>Q: Публикация telemetry.raw.<device_id> (Msg-Id = event_id)
    I-->>D: 202 Accepted (batch_id, accepted_count)

    Q->>P: Доставка пакета воркеру
    P->>R: Проверка дедупликации (SETNX dedup:device:metric:ts)
    P->>DB: Пакетная вставка измерений (SendBatch ON CONFLICT DO NOTHING)
    P->>R: Обновление актуального состояния (HSET device:id:metrics, status=ONLINE)
    P->>DB: Обновление last_seen_at устройства

    P->>DB: Запрос активных правил для полученных метрик
    alt Условие правила выполнено (например, Temp > 80)
        P->>DB: Проверка наличия активного алерта (подавление дублей)
        alt Активного инцидента нет
            P->>DB: Создание Alert (FIRING) + AlertEvent
            P->>Q: Публикация telemetry.alerts
        end
    else Значение вернулось в норму и был активный алерт
        P->>DB: Закрытие инцидента (status=RESOLVED, resolved_at=NOW())
        P->>Q: Публикация telemetry.alerts (RESOLVED)
    end
    P-->>Q: JetStream Ack()

    Q->>W: Доставка события алерта
    W->>WH: Отправка HTTP POST на Webhook URL (с повторами при ошибках)
    W->>DB: Запись результата в webhook_deliveries
    W-->>Q: JetStream Ack()
```

---

## 🗄️ Модель данных (ER-диаграмма)

```mermaid
erDiagram
    devices ||--o{ device_credentials : "has"
    devices ||--o{ telemetry_batches : "receives"
    devices ||--o{ measurements : "emits"
    devices ||--o{ alert_rules : "targets"
    devices ||--o{ alerts : "raises"

    alert_rules ||--o{ alerts : "triggers"
    alerts ||--o{ alert_events : "tracks"
    alerts ||--o{ webhook_deliveries : "dispatches"

    devices {
        string id PK
        string name
        string group_name
        string status
        timestamptz last_seen_at
        timestamptz created_at
        timestamptz updated_at
    }

    device_credentials {
        uuid id PK
        string device_id FK
        string key_hash
        string key_prefix
        boolean revoked
        timestamptz issued_at
        timestamptz revoked_at
    }

    metric_definitions {
        uuid id PK
        string name UK
        string display_name
        string data_type
        string unit
        float min_value
        float max_value
        timestamptz created_at
    }

    measurements {
        bigserial id
        string device_id FK
        string metric
        float value
        string unit
        timestamptz recorded_at
        timestamptz ingested_at
    }

    alert_rules {
        uuid id PK
        string name
        string device_id FK
        string group_name
        string metric
        string operator
        float threshold
        int duration_seconds
        string severity
        boolean active
        text webhook_url
        timestamptz created_at
        timestamptz updated_at
    }

    alerts {
        uuid id PK
        uuid rule_id FK
        string device_id FK
        string metric
        float trigger_value
        float threshold
        string status
        text message
        timestamptz fired_at
        timestamptz acknowledged_at
        timestamptz resolved_at
        timestamptz created_at
        timestamptz updated_at
    }

    alert_events {
        uuid id PK
        uuid alert_id FK
        string old_status
        string new_status
        float trigger_value
        text reason
        timestamptz created_at
    }

    webhook_deliveries {
        uuid id PK
        uuid alert_id FK
        uuid alert_event_id FK
        text url
        int attempt
        string status
        int response_code
        text error_message
        timestamptz delivered_at
        timestamptz created_at
    }
```

---

## 🛠️ Стек технологий и принятые решения

| Технология | Назначение | Обоснование выбора |
|---|---|---|
| **Go 1.26** | Основной язык платформы | Высокая скорость работы, конкурентность на горутинах, минимальное потребление RAM. |
| **net/http (Go 1.22+)** | REST API маршрутизация | Встроенное сопоставление путей и методов. Нулевые накладные расходы, отказ от сторонних тяжелых роутеров. |
| **gRPC & Protocol Buffers** | Бинарный транспорт телеметрии | Сверхкомпактная сериализация и потоковая передача (`SendStream`) для высоконагруженных агентов. |
| **NATS JetStream** | Потоковый брокер сообщений | Субмиллисекундная задержка, минимальный футпринт памяти, персистентность, поддержка DLQ и at-least-once. |
| **PostgreSQL 16** | Долговременное хранилище | Надежность ACID, пакетные вставки `pgx.Batch`, B-Tree индексы на `(device, metric, time)` и `date_bin` агрегация. |
| **pgx/v5 (pgxpool)** | Драйвер БД | Максимальная производительность среди драйверов Go, прямой контроль пула и отсутствие прослоек ORM. |
| **Redis 7** | Оперативный кэш и дедупликация | Хранение текущих снимков датчиков в хэшах, атомарный sliding-window rate limiting и дедупликация через `SETNX`. |
| **Prometheus & pprof** | Наблюдаемость и профилирование | Экспорт метрик латентности, объёмов инжеста, счётчиков инцидентов и встроенный профайлер CPU/памяти. |

---

## 🚀 Быстрый старт

### Требования
- Установленный **Docker** и **Docker Compose**
- **Go 1.22+** (если запуск производится локально без Docker)

### 1. Запуск полного стека через Docker Compose

```bash
docker compose up --build -d
```
Команда поднимет:
- PostgreSQL 16 (`:5432`)
- Redis 7 (`:6379`)
- NATS JetStream (`:4222`, `:8222`)
- DevicePulse All-in-One сервис (`:8080`, `:50051`)

После старта откройте панель управления:
👉 **[http://localhost:8080/](http://localhost:8080/)**

### 2. Локальный запуск сервиса в Go

```bash
# 1. Запуск баз данных в Docker
docker compose up -d postgres redis nats

# 2. Запуск сервиса DevicePulse
$env:DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/devicepulse?sslmode=disable"
go run main.go
```

---

## ⚡ Нагрузочное тестирование (`load-generator`)

В проект включен встроенный генератор нагрузки `cmd/load-generator`. Он автоматически регистрирует тестовые устройства, получает ключи и генерирует потоки измерений с замером перцентилей задержки:

```bash
# Стресс-тест REST API (50 устройств, батчи по 20 измерений, длительность 30 секунд):
go run ./cmd/load-generator -transport=rest -workers=50 -batch-size=20 -duration=30s -target=http://localhost:8080

# Высоконагруженный тест gRPC:
go run ./cmd/load-generator -transport=grpc -workers=50 -batch-size=50 -duration=30s -target=localhost:50051
```

Пример вывода бенчмарка:
```text
================ LOAD TEST RESULTS ================
Elapsed Time:         30.01s
Total Requests:       15000
Successful Requests:  15000 (100.00%)
Total Measurements:   300000
Throughput (Req/s):   499.82 req/s
Throughput (Data/s):  9996.40 measurements/s
----------------- LATENCY DISTRIBUTION -----------------
Min:                  1.12ms
Avg:                  4.85ms
p50 (Median):         4.10ms
p95:                  9.62ms
p99:                  14.80ms
Max:                  28.40ms
====================================================
```

---

## 🧪 Тестирование

```bash
# Запуск всех модульных тестов
go test -v ./...

# Запуск с проверкой состояния гонок (Race Detector)
go test -race ./...
```

---

## 📜 Лицензия

Проект распространяется под свободной лицензией [MIT](LICENSE).
