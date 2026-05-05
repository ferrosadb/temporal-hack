-- Telemetry database schema. Runs once at first lab bring-up.
-- The platform installer is responsible for running this against the
-- target Postgres before any service tries to connect.

CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS telemetry (
    robot_id    text        NOT NULL,
    stream      text        NOT NULL,
    captured_at timestamptz NOT NULL,
    payload     bytea       NOT NULL,
    schema      text        NOT NULL DEFAULT '',
    PRIMARY KEY (robot_id, stream, captured_at)
);

-- Make it a hypertable partitioned by captured_at (1-day chunks).
-- Idempotent: SELECT returns the existing hypertable if already created.
SELECT create_hypertable('telemetry', 'captured_at',
                         chunk_time_interval => INTERVAL '1 day',
                         if_not_exists       => TRUE);

CREATE INDEX IF NOT EXISTS idx_telemetry_robot_stream_ts
    ON telemetry (robot_id, stream, captured_at DESC);

-- v1 retention: 30 days. Tune at S2 once cardinality is measured.
SELECT add_retention_policy('telemetry', INTERVAL '30 days', if_not_exists => TRUE);
