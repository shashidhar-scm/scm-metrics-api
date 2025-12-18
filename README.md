# metrics-api

A small Go HTTP service that ingests Telegraf metrics and stores them in TimescaleDB/Postgres.

It supports:

- A **summary table** (`server_metrics`) for simple dashboard cards and charts.
- A **metric-per-series table** (`metric_points`) for flexible dashboards (query any curated series by measurement/field and optional tag filters).

## Requirements

- Go (for local run)
- Postgres/TimescaleDB

## Run locally (Go)

The service auto-creates the DB and schema on startup.

### Environment variables

- `DATABASE_URL` (optional; if set, overrides all DB_* variables)

Otherwise it builds the connection string from:

- `PSQL_HOST` (default: `localhost`)
- `PSQL_PORT` (default: `5432`)
- `PSQL_USER` (default: `postgres`)
- `PSQL_PASSWORD` (default: `secret`)
- `PSQL_DB_NAME` (default: `metrics`)
- `PSQL_SSLMODE` (default: `disable`)
- `PSQL_ADMIN_DB` (default: `postgres`) (used to create `PSQL_DB_NAME` if missing)

Optional:

- `DEBUG` (set to any non-empty value to enable ingest debug logging)

### Run

```bash
go run main.go
```

## Run with Docker Compose (TimescaleDB)

A compose file is included to run:

- TimescaleDB on `localhost:5432`
- metrics-api on `localhost:8080`

```bash
docker compose up --build
```

## Telegraf setup

This repo includes `metrics.sh` to install/configure Telegraf and send metrics to this API via HTTP output.

- It configures Telegraf to POST to:
  - `https://scm-metrics-api.citypost.us/api/metrics` (as currently set in the script)

If you are running locally, change `API_URL` in `metrics.sh` to:

- `http://localhost:8080/api/metrics`

## API

All routes are served on port `8080`.

### Health

- `GET /`
  - Returns: `Application is up and running`

### Summary endpoints (from `server_metrics`)

- `POST /api/metrics`
  - Ingest Telegraf JSON payload.
  - Inserts one summary row into `server_metrics`.
  - Also writes curated per-series points into `metric_points`.

- `GET /api/servers`
  - Returns list of servers.

- `GET /api/metrics/latest`
  - Returns latest summary row per server.

- `GET /api/metrics/history?server_id=<id>&range=<interval>`
  - Returns summary points for a server within a time range.
  - `range` examples: `10m`, `1h`, `6h`, `1d`.

### Series endpoints (from `metric_points`)

These endpoints enable a dashboard to query a curated set of series.

- `GET /api/series?server_id=<id>`
  - Lists available `(measurement, field)` pairs for that server.

- `GET /api/series/latest?server_id=<id>&measurement=<m>&field=<f>&tags=<json>`
  - Returns the latest point for a series.
  - `tags` is optional JSON used for filtering via Postgres JSONB `@>`.

- `GET /api/series/query?server_id=<id>&measurement=<m>&field=<f>&range=<interval>&tags=<json>`
  - Returns time-ordered points for a series in the requested range.
  - `tags` is optional JSON.

#### Tag filter examples

`tags` must be URL-encoded JSON.

- CPU total only:
  - `tags={"cpu":"cpu-total"}`

- Aggregated disk series:
  - `tags={"aggregated":true}`

## Curated subset written to `metric_points`

The ingest stores only a curated subset:

- `cpu` (only `cpu=cpu-total`)
  - `usage_user`, `usage_system`, `usage_iowait`, `usage_steal`, `usage_idle`
- `mem`
  - `available_percent`, `used_percent`, `total`, `used`
- `swap`
  - `used_percent`, `in`, `out`
- `system`
  - `load1`, `load5`, `load15`, `uptime`
- `processes`
  - `running`, `blocked`, `zombies`, `total`
- `disk` (aggregated across all real filesystems)
  - `total`, `used`, `free`, `used_percent` with tags `{"aggregated":true}`
- `diskio` (all devices)
  - `read_bytes`, `write_bytes`, `io_util`, `io_await`

## CORS

CORS is enabled globally:

- `Access-Control-Allow-Origin: *`
- Preflight `OPTIONS` returns `204`

## Docker build (Kubernetes/GKE)

The Dockerfile uses a builder image and a distroless runtime image.

To avoid `exec format error` on GKE, build for the correct platform:

```bash
docker buildx build --platform linux/amd64 -t <repo>/scm-metrics-api:<tag> --push .
```

Verify the cluster nodes are the expected architecture and that your Deployment is using the image/tag you just pushed:

```bash
kubectl get nodes -o wide
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.nodeInfo.architecture}{"\n"}{end}'

kubectl get deploy scm-metrics-api -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
```

After pushing a corrected image, restart the Deployment and watch the rollout/logs:

```bash
kubectl rollout restart deploy/scm-metrics-api
kubectl rollout status deploy/scm-metrics-api
kubectl logs -f deploy/scm-metrics-api
```

## Dashboard (Angular)

A minimal Angular dashboard is included under `dashboard/`.

It uses a dev proxy so frontend requests to `/api/*` are proxied to `http://localhost:8080`.

### Run

Start the Go API locally on `:8080`, then:

```bash
cd dashboard
npm install
npm start
```

Open:

- `http://localhost:4200`

## Notes

- Schema auto-migration happens on startup. The DB user must have permission to create the target database (first run) and create tables/extensions.
