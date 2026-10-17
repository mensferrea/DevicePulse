# DevicePulse — Платформа сбора, обработки и мониторинга телеметрии устройств

DevicePulse — высокопроизводительный backend на Go для сбора, конкурентной потоковой обработки, долговременного хранения и анализа телеметрии от IoT-устройств, программных агентов и физических микроконтроллеров.

Система спроектирована по принципам чистой архитектуры (Clean / Hexagonal Architecture) с разделением на независимые слои, нулевой связностью бизнес-логики с деталями инфраструктуры, поддержкой REST и gRPC протоколов, асинхронным контуром на NATS JetStream, кэшированием актуального состояния в Redis и сохранением исторических временных рядов в PostgreSQL.

---

## 1. Архитектура системы

Платформа представляет собой модульный сервис, компоненты которого могут работать совместно в одном процессе (`all-in-one`) либо масштабироваться независимо в виде отдельных микросервисов:

- **Ingest API**: Приём телеметрии через HTTP REST (`POST /api/v1/telemetry`, `POST /api/v1/telemetry/batch`) и gRPC (`TelemetryService.SendBatch`, `TelemetryService.SendStream`). Выполняет аутентификацию по криптографическим ключам, валидацию временных меток/границ значений, ограничение частоты запросов (rate limiting) и публикует принятые пакеты в NATS JetStream.
- **Control API**: REST API для администрирования: регистрация устройств, выпуск и отзыв API-ключей, управление описаниями метрик, настройка правил оповещений, подтверждение и закрытие инцидентов, запрос агрегированной статистики и временных рядов.
- **Telemetry Processor**: Асинхронный воркер-пул потребителей NATS JetStream. Выполняет проверку идемпотентности, пакетную вставку измерений в PostgreSQL, обновление актуального состояния устройств в Redis, вычисление статуса доступности (`ONLINE`, `OFFLINE`, `UNKNOWN`) и оценку пороговых правил алертинга.
- **Alert Worker**: Потребитель событий инцидентов из JetStream. Обеспечивает надёжную доставку webhook-уведомлений с повторными попытками (exponential backoff) и сохранением статусов доставки в БД.

### Архитектурная диаграмма компонентов

```mermaid
flowchart TD
    subgraph Clients["Источники данных и клиенты"]
        A[IoT Устройства / Агенты]
        B[Оператор / Панель мониторинга]
        C[Внешние системы / Webhook Receiver]
    end

    subgraph TransportLayer["Слой приёма (Ingest & Control)"]
        D[REST Ingest API]
        E[gRPC Ingest Server]
        F[Control REST API]
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
    B -->|REST Management| F

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

---

## 2. Диаграмма последовательности приёма и обработки телеметрии

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

## 3. Модели состояний (State Machines)

### Жизненный цикл доступности устройства

```mermaid
stateDiagram-v2
    [*] --> UNKNOWN: Регистрация устройства
    UNKNOWN --> ONLINE: Получено первое измерение
    ONLINE --> ONLINE: Поступают новые измерения (обновление last_seen)
    ONLINE --> OFFLINE: Простой > OFFLINE_TIMEOUT (по фоновому сканеру)
    OFFLINE --> ONLINE: Поступило новое измерение
```

### Жизненный цикл предупреждения (Alert)

```mermaid
stateDiagram-v2
    [*] --> FIRING: Условие правила выполнено (порог нарушен)
    FIRING --> ACKNOWLEDGED: Подтверждено оператором (POST /api/v1/alerts/{id}/ack)
    FIRING --> RESOLVED: Показатели нормализовались (автоматически)
    FIRING --> RESOLVED: Ручное закрытие оператором
    ACKNOWLEDGED --> RESOLVED: Показатели нормализовались (автоматически)
    ACKNOWLEDGED --> RESOLVED: Ручное закрытие оператором
    RESOLVED --> [*]
```

---

## 4. Модель данных (ER-диаграмма)

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

## 5. Обоснование технических решений (Rationale)

### 1. Архитектурный стиль: Чистая архитектура и модульный монолит
- **Почему так:** Бизнес-логика (`internal/domain` и `internal/service`) полностью отвязана от внешних библиотек и базы данных через интерфейсы репозиториев (`DeviceRepository`, `TelemetryRepository`, `StateRepository` и т.д.).
- **Преимущество:** Это позволяет запускать всю платформу в режиме `all-in-one` (удобно для разработки, сквозных тестов и небольших инсталляций) либо собирать независимые бинарники из `cmd/` (`ingest-api`, `control-api`, `telemetry-processor`, `alert-worker`) для горизонтального масштабирования под высокой нагрузкой.

### 2. NATS JetStream как событийная шина
- **Почему не обычный RabbitMQ или Kafka:** NATS написан на Go, потребляет минимальное количество памяти, обладает ультранизкой задержкой (sub-millisecond latency) и встроенной поддержкой персистентности (JetStream).
- **Гарантии доставки:** Использована семантика *At-Least-Once* с подтверждением (`Ack()`), отложенным повтором (`NakWithDelay`) при временных сбоях БД, и защитой от неисправимых сообщений (poison pills) через ограничение числа доставок (`MaxDeliver: 5`) и маршрутизацию в `telemetry.dlq`.

### 3. PostgreSQL: pgx/v5, pgxpool и чистый SQL
- **Почему не ORM (GORM/Ent):** ORM создают накладные расходы при высокой частоте вставок, скрывают нюансы работы транзакций и усложняют пакетные операции.
- **Оптимизация:** Применён `pgx.Batch` (`SendBatch`) для групповой вставки сотен измерений за один сетевой round-trip к БД с конструкцией `ON CONFLICT (device_id, metric, recorded_at) DO NOTHING`.
- **Индексы и агрегаты:** Спроектированы B-Tree индексы на `(device_id, metric, recorded_at DESC)`. Расчёт минутных и пятиминутных окон производится через встроенную функцию `date_bin()`, что на порядки быстрее выборки сырых данных в приложение.

### 4. Redis: In-Memory состояние, Rate Limiting и Дедупликация
- **Разделение ответственности:** История и транзакционные данные живут в PostgreSQL. Redis используется только как быстрый буфер оперативного состояния:
  - `HSET device:<id>:metrics` — мгновенное чтение последних значений датчиков без обращения к тяжелой таблице измерений.
  - `SETNX dedup:<device>:<metric>:<ts>` с TTL 24 часа — мгновенная фильтрация дублирующих измерений до записи в диск.
  - Атомарный инкремент с окном для Rate Limiting устройств.
- **Отказоустойчивость:** При потере Redis платформа продолжает принимать и сохранять данные; актуальное состояние может быть восстановлено из последних записей PostgreSQL.

### 5. HTTP-роутер на стандартной библиотеке Go 1.22+
- Начиная с Go 1.22 `net/http.ServeMux` поддерживает сопоставление HTTP-методов и параметров пути (`POST /api/v1/devices/{id}`). Это устранило потребность во внешних тяжелых фреймворках, гарантирует максимальную производительность, нулевую зависимость от сторонних HTTP-библиотек и стандартную совместимость.

### 6. gRPC и Protocol Buffers
- Компактный бинарный формат Protobuf минимизирует накладные расходы на сериализацию и сетевой трафик, что критично для миллионов измерений от сотен устройств. Предоставлены контракты пакетной отправки (`SendBatch`) и клиентского стриминга (`SendStream`).

---

## 6. Запуск и развертывание

### Требования
- Go 1.22+
- Docker и Docker Compose

### 1. Быстрый запуск полного контура через Docker Compose

```bash
docker compose up --build -d
```

Команда запустит:
- PostgreSQL 16 (порт `5432`)
- Redis 7 (порт `6379`)
- NATS JetStream (порты `4222`, `8222`)
- Контейнер `devicepulse-app` (`all-in-one` сервис: REST `8080`, gRPC `50051`)

Миграции базы данных применяются автоматически при старте приложения.

### 2. Локальный запуск без контейнеризации приложения

Если Postgres, Redis и NATS уже запущены:

```bash
# Запуск единого процесса (All-in-One)
go run main.go
```

Или запуск отдельных микросервисов:

```bash
go run ./cmd/control-api
go run ./cmd/ingest-api
go run ./cmd/telemetry-processor
go run ./cmd/alert-worker
```

---

## 7. Генератор нагрузки и стресс-тестирование

В проект включён профессиональный генератор синтетической телеметрии `cmd/load-generator`.

### Возможности:
- Автоматическая предварительная регистрация тестовых устройств и выпуск API-ключей.
- Конкурентная генерация трафика через настраиваемое число воркеров.
- Поддержка обоих протоколов: HTTP REST и gRPC.
- Настраиваемый размер батча и ограничение RPS.
- Расчёт пропускной способности (запросов/сек и измерений/сек).
- Измерение распределения задержки: Min, Avg, p50 (медиана), p95, p99, Max.

### Примеры запуска:

```bash
# Нагрузочное тестирование REST API (50 параллельных устройств, батчи по 20 измерений, длительность 30 секунд)
go run ./cmd/load-generator -transport=rest -workers=50 -batch-size=20 -duration=30s -target=http://localhost:8080

# Высоконагруженное тестирование через gRPC
go run ./cmd/load-generator -transport=grpc -workers=50 -batch-size=50 -duration=30s -target=localhost:50051
```

### Мониторинг и профилирование:
- **Метрики Prometheus**: `GET http://localhost:8080/metrics`
- **Профилирование pprof**:
  - `GET http://localhost:8080/debug/pprof/`
  - `go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30`
  - `go tool pprof http://localhost:8080/debug/pprof/heap`

---

## 8. Запуск unit- и интеграционных тестов

```bash
# Запуск всех тестов
go test -v ./...

# Запуск с проверкой гонок (race detector)
go test -race ./...
```

---

## 9. Документация API

- OpenAPI 3.0 спецификация находится в [`api/openapi/openapi.yaml`](api/openapi/openapi.yaml).
- Protobuf контракты находятся в [`api/proto/telemetry.proto`](api/proto/telemetry.proto).
