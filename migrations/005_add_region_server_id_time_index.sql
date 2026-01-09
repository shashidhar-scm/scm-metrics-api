-- Add region-aware index to speed up latest-per-server queries filtered by region
-- Note: TimescaleDB hypertables do not support CREATE INDEX CONCURRENTLY

CREATE INDEX IF NOT EXISTS idx_server_metrics_region_server_id_time_desc
  ON server_metrics (region, server_id, time DESC);
