# scm-metrics-api WORKLOG

This log explains the purpose of the service and captures meaningful changes so future agents understand context before editing.

## Service overview
- Go HTTP API that ingests Telegraf metrics (`POST /api/metrics`) and stores them in TimescaleDB/Postgres via `server_metrics` + `metric_points` tables (@scm-metrics-api/README.md#1-192).
- Exposes summary dashboards (`GET /api/servers`, `/api/metrics/latest`, `/api/metrics/history`) and curated series endpoints (`/api/series/*`).
- Ships with Docker compose, Kubernetes manifests, and an Angular dashboard under `dashboard/` for quick visualization.

## 2025-12-29
- Refactored the service into packages:
  - `internal/models` for all shared structs (moved from `main.go`).
  - `internal/db` now handles config, DB creation, and schema bootstrap (Timescale checks + hypertables).
  - `internal/repository` wraps every SQL query in a `MetricsRepository`.
  - `internal/handlers` contains `MetricsHandler`, which owns all HTTP endpoints and uses the repo.
  - `internal/routes` wires handler methods + middleware.
- `main.go` now only wires: DB setup → repository → handler → routes + rate limiter, making the entry point minimal.
- Added batched `metricPoints` flush logic to call `MetricsRepository.SaveSeriesPoints` instead of raw SQL.
- README updated with new project structure and rate-limiter docs; Docker/K8s notes unchanged.
- Reminder: run `gofmt`/`go test` (not run in this environment) after changes.
