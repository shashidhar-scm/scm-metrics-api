# scm-metrics-api WORKLOG

This log explains the purpose of the service and captures meaningful changes so future agents understand context before editing.

## Service overview
- Go HTTP API that ingests Telegraf metrics (`POST /api/metrics`) and stores them in TimescaleDB/Postgres via `server_metrics` + `metric_points` tables (@scm-metrics-api/README.md#1-192).
- Exposes summary dashboards (`GET /api/servers`, `/api/metrics/latest`, `/api/metrics/history`) and curated series endpoints (`/api/series/*`).
- Ships with Docker compose, Kubernetes manifests, and an Angular dashboard under `dashboard/` for quick visualization.

## 2025-12-29
- Reviewed README to ensure environment/deployment docs are accurate; no code changes yet, but this entry documents that the gateway tooling now depends on `/metrics/*` endpoints being accessible via tool-gateway-http. Future edits should keep `/api/metrics/latest` and `/api/metrics/history` stable so existing dashboards keep working.
- Added an in-memory, IP-based rate limiter that wraps every HTTP handler. Configuration is via `RATE_LIMIT_WINDOW_SECONDS` and `RATE_LIMIT_MAX` env vars (default 60s / 120 requests) so deployments can throttle noisy agents without code changes (@scm-metrics-api/main.go#1-1203).
- Updated `saveMetric` to use `ON CONFLICT (server_id, time) DO UPDATE` to eliminate duplicate-key errors when rapid re-ingests hit the same timestamp, keeping the latest CPU/memory/disk snapshot instead of failing (@scm-metrics-api/main.go#715-737).
