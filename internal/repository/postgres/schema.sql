CREATE TABLE IF NOT EXISTS devices (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    group_name VARCHAR(128) NOT NULL DEFAULT 'default',
    status VARCHAR(32) NOT NULL DEFAULT 'UNKNOWN',
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_devices_group ON devices(group_name);
CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen_at);

CREATE TABLE IF NOT EXISTS device_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id VARCHAR(64) NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(16) NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dev_cred_dev_id ON device_credentials(device_id);
CREATE INDEX IF NOT EXISTS idx_dev_cred_hash ON device_credentials(key_hash) WHERE revoked = FALSE;

CREATE TABLE IF NOT EXISTS metric_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) UNIQUE NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    data_type VARCHAR(32) NOT NULL DEFAULT 'gauge',
    unit VARCHAR(32) NOT NULL,
    min_value DOUBLE PRECISION,
    max_value DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS telemetry_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id VARCHAR(64) NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    batch_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'RECEIVED',
    received_count INT NOT NULL DEFAULT 0,
    processed_count INT NOT NULL DEFAULT 0,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT uq_device_batch UNIQUE (device_id, batch_id)
);

CREATE TABLE IF NOT EXISTS measurements (
    id BIGSERIAL NOT NULL,
    device_id VARCHAR(64) NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    metric VARCHAR(64) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    unit VARCHAR(32) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (device_id, metric, recorded_at, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_measurements_dedup 
ON measurements(device_id, metric, recorded_at);

CREATE INDEX IF NOT EXISTS idx_measurements_query 
ON measurements(device_id, metric, recorded_at DESC);

CREATE INDEX IF NOT EXISTS idx_measurements_recorded_at 
ON measurements(recorded_at);

CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    device_id VARCHAR(64) REFERENCES devices(id) ON DELETE CASCADE,
    group_name VARCHAR(128),
    metric VARCHAR(64) NOT NULL,
    operator VARCHAR(8) NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    duration_seconds INT NOT NULL DEFAULT 0,
    severity VARCHAR(32) NOT NULL DEFAULT 'warning',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    webhook_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_metric_active 
ON alert_rules(metric) WHERE active = TRUE;

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    device_id VARCHAR(64) NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    metric VARCHAR(64) NOT NULL,
    trigger_value DOUBLE PRECISION NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    message TEXT NOT NULL DEFAULT '',
    fired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_active_lookup 
ON alerts(device_id, rule_id, status);

CREATE INDEX IF NOT EXISTS idx_alerts_status_fired 
ON alerts(status, fired_at DESC);

CREATE TABLE IF NOT EXISTS alert_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    old_status VARCHAR(32) NOT NULL,
    new_status VARCHAR(32) NOT NULL,
    trigger_value DOUBLE PRECISION NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_events_alert 
ON alert_events(alert_id, created_at DESC);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    alert_event_id UUID REFERENCES alert_events(id) ON DELETE SET NULL,
    url TEXT NOT NULL,
    attempt INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    response_code INT,
    error_message TEXT,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_alert_status 
ON webhook_deliveries(alert_id, status);
