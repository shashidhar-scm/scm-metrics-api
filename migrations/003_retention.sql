DO $$
BEGIN
    -- Metric points retention: keep 60 days
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'metric_points'
        AND proc_name = 'policy_retention'
    ) THEN
        PERFORM add_retention_policy('metric_points', INTERVAL '60 days');
    END IF;

    -- Server metrics rollup retention: keep 365 days
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'server_metrics'
        AND proc_name = 'policy_retention'
    ) THEN
        PERFORM add_retention_policy('server_metrics', INTERVAL '365 days');
    END IF;
END$$;
