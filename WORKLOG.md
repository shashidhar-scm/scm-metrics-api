# scm-metrics-api WORKLOG

This log explains the purpose of the service and captures meaningful changes so future agents understand context before editing.

## Service overview
- Go HTTP API that ingests Telegraf metrics (`POST /api/metrics`) and stores them in TimescaleDB/Postgres via `server_metrics` + `metric_points` tables (@scm-metrics-api/README.md#1-192).
- Exposes summary dashboards (`GET /api/servers`, `/api/metrics/latest`, `/api/metrics/history`) and curated series endpoints (`/api/series/*`).
- Ships with Docker compose, Kubernetes manifests, and an Angular dashboard under `dashboard/` for quick visualization.

## 2025-12-29
- Reviewed README to ensure environment/deployment docs are accurate; no code changes yet, but this entry documents that the gateway tooling now depends on `/metrics/*` endpoints being accessible via tool-gateway-http. Future edits should keep `/api/metrics/latest` and `/api/metrics/history` stable so existing dashboards keep working.
