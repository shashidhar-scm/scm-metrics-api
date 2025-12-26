DO $$
BEGIN
    IF NOT EXISTS (
        SELECT FROM pg_database WHERE datname = 'metrics'
    ) THEN
        EXECUTE 'CREATE DATABASE metrics';
    END IF;
END$$;
