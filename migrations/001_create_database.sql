SELECT 'CREATE DATABASE metrics'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'metrics')\gexec
