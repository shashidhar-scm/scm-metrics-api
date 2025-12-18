CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS server_metrics (
  time TIMESTAMPTZ NOT NULL,
  server_id TEXT NOT NULL,
  cpu DOUBLE PRECISION NOT NULL DEFAULT 0,
  memory DOUBLE PRECISION NOT NULL DEFAULT 0,
  memory_total_bytes BIGINT NOT NULL DEFAULT 0,
  memory_used_bytes BIGINT NOT NULL DEFAULT 0,
  disk DOUBLE PRECISION NOT NULL DEFAULT 0,
  disk_total_bytes BIGINT NOT NULL DEFAULT 0,
  disk_used_bytes BIGINT NOT NULL DEFAULT 0,
  disk_free_bytes BIGINT NOT NULL DEFAULT 0,
  uptime BIGINT NOT NULL DEFAULT 0,
  city TEXT,
  city_name TEXT,
  region TEXT,
  region_name TEXT
);

CREATE INDEX IF NOT EXISTS idx_server_metrics_server_id_time_desc
  ON server_metrics (server_id, time DESC);

CREATE INDEX IF NOT EXISTS idx_server_metrics_time
  ON server_metrics (time DESC);

SELECT create_hypertable('server_metrics', 'time', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS metric_points (
  time TIMESTAMPTZ NOT NULL,
  server_id TEXT NOT NULL,
  measurement TEXT NOT NULL,
  field TEXT NOT NULL,
  value_double DOUBLE PRECISION NULL,
  value_int BIGINT NULL,
  tags JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_metric_points_series_time_desc
  ON metric_points (server_id, measurement, field, time DESC);

SELECT create_hypertable('metric_points', 'time', if_not_exists => TRUE);
