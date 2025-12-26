------------------------------------------------------------------------
-- Enable compression for hypertables
------------------------------------------------------------------------

-- metric_points (raw data)
ALTER TABLE metric_points SET (timescaledb.compress);
-- default segmentby: server_id
-- default orderby: "time" DESC

-- server_metrics (rollups)
ALTER TABLE server_metrics SET (timescaledb.compress);
-- default segmentby: server_id
-- default orderby: "time" DESC


------------------------------------------------------------------------
-- Add retention + compression policies (idempotent)
------------------------------------------------------------------------
DO $$
BEGIN
    ------------------------------------------------------------------------
    -- metric_points: raw high-resolution data
    ------------------------------------------------------------------------
    -- Retain only latest 60 days
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'metric_points'
        AND proc_name = 'policy_retention'
    ) THEN
        PERFORM add_retention_policy('metric_points', INTERVAL '60 days');
    END IF;

    -- Compress chunks older than 7 days
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'metric_points'
        AND proc_name = 'policy_compression'
    ) THEN
        PERFORM add_compression_policy('metric_points', INTERVAL '7 days');
    END IF;


    ------------------------------------------------------------------------
    -- server_metrics: rollup data
    ------------------------------------------------------------------------
    -- Retain 365 days
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'server_metrics'
        AND proc_name = 'policy_retention'
    ) THEN
        PERFORM add_retention_policy('server_metrics', INTERVAL '365 days');
    END IF;

    -- Compress chunks older than 14 days
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'server_metrics'
        AND proc_name = 'policy_compression'
    ) THEN
        PERFORM add_compression_policy('server_metrics', INTERVAL '14 days');
    END IF;

END$$;


------------------------------------------------------------------------
-- (Optional) Add maintenance job – weekly VACUUM ANALYZE
-- uncomment if using TimescaleDB job scheduler
------------------------------------------------------------------------
-- DO $$
-- BEGIN
--     IF NOT EXISTS (
--         SELECT 1 FROM timescaledb_information.jobs
--         WHERE proc_name = 'user_defined_job' 
--         AND (config->>'command') = 'VACUUM ANALYZE'
--     ) THEN
--         PERFORM add_job('VACUUM ANALYZE', schedule_interval => INTERVAL '7 days');
--     END IF;
-- END$$;


------------------------------------------------------------------------
-- Verification Queries
------------------------------------------------------------------------

-- View which policies/jobs were added
-- SELECT job_id, hypertable_name, proc_name, schedule_interval, config
-- FROM timescaledb_information.jobs
-- ORDER BY job_id;

-- View compression configuration
-- SELECT hypertable_name, compress_orderby, compress_segmentby
-- FROM timescaledb_information.hypertables;

-- Manually force compress (for testing)
-- SELECT compress_chunk('_timescaledb_internal._hyper_1_1_chunk');
