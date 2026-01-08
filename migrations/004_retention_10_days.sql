DO $$
BEGIN
    -- Ensure metric_points retention keeps only 10 days of data
    IF EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'metric_points'
          AND proc_name = 'policy_retention'
    ) THEN
        PERFORM remove_retention_policy('metric_points');
    END IF;
    PERFORM add_retention_policy('metric_points', INTERVAL '10 days');

    -- Ensure server_metrics retention keeps only 10 days of data
    IF EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE hypertable_name = 'server_metrics'
          AND proc_name = 'policy_retention'
    ) THEN
        PERFORM remove_retention_policy('server_metrics');
    END IF;
    PERFORM add_retention_policy('server_metrics', INTERVAL '10 days');
END$$;
