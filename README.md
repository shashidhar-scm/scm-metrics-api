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

### Project structure

```text
internal/
  db/           # Connection config + schema/bootstrap logic
  models/       # All request/response + persistence structs
  repository/   # MetricsRepository encapsulating DB access
  handlers/     # MetricsHandler with HTTP endpoints
  routes/       # Router helpers for wiring handlers + middleware
```

`main.go` now only wires together the DB setup, repository, handler, middleware, and routes, keeping business logic inside the packages above.

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
- `DEBUG_SERVER_ID` (optional; when set alongside `DEBUG`, only log payload/metric details for that specific server ID or host tag)

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

## Telegraf installer (.deb)

Kiosks now install the Telegraf stack via a Debian package that wraps the old `metrics.sh` logic.

### Layout

```
installer/
  bin/        # helper scripts copied to /usr/local/bin
  configs/    # telegraf.d fragments + templates (envsubst)
  install.sh  # main entrypoint (also lives at /opt/scm-metrics/install.sh)
  DEBIAN/     # control/postinst/prerm for dpkg
```

`metrics.sh` in the repo is now just a thin wrapper that execs `installer/install.sh`, so kiosks that still download the raw script continue to work.

### Build the installer

```bash
# Local build (requires dpkg-deb installed)
make installer-deb

# Or build inside Docker (handy on macOS)
make installer-deb-docker
```

The output lives under `build/` (e.g. `build/scm-metrics-installer_0.0.0+dev.deb`). Set `VERSION=1.2.3` on the command line or tag the repo so the package version starts with a digit (Debian requirement).

### Install / upgrade on a kiosk

```bash
scp build/scm-metrics-installer_<ver>.deb smartcity@kiosk:/tmp/
ssh smartcity@kiosk
sudo systemctl stop telegraf   # ensure the old metrics.sh isn’t running
sudo dpkg -i /tmp/scm-metrics-installer_<ver>.deb
```

The `postinst` script runs `/opt/scm-metrics/install.sh`, which:

1. Installs/updates Telegraf, vnStat, jq, lm-sensors, alsa-utils, etc.
2. Copies helper scripts into `/usr/local/bin`.
3. Renders templated configs (HTTP output, inputs, global tags) into `/etc/telegraf/telegraf.d`.
4. Validates config and restarts Telegraf.

### Validation checklist

1. `sudo journalctl -u telegraf -n 50` – ensure no HTTP output errors.
2. `sudo telegraf --config /etc/telegraf/telegraf.conf --config-directory /etc/telegraf/telegraf.d --test | tail` – confirm helpers emit line protocol.
3. `curl -v https://scm-metrics-api.citypost.us/api/metrics` (expect 405/404) – basic reachability.
4. After one minute, hit `/api/metrics/latest?server_id=<id>` and verify new fields like `link_state` and `process_statuses` are present.

For local testing, override `API_URL` when running the installer (`API_URL=http://localhost:8080/api/metrics make installer-deb && sudo API_URL=... /opt/scm-metrics/install.sh`).

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
  - Query params: `page`, `page_size` (default `25`, max `200`)

- `GET /api/servers/status`
  - Returns online/offline status per server (threshold controls freshness).
  - Query params: `page`, `page_size`.

- `GET /api/servers/status/city?region=<region>&threshold=<interval>`
  - Aggregates counts per city (online/offline/total).
  - Query params: `page`, `page_size`.

- `GET /api/metrics/history?server_id=<id>&range=<interval>`
  - Returns summary points for a server within a time range.
  - `range` examples: `10m`, `1h`, `6h`, `1d`.
  - Supports `page`, `page_size`.

### Series endpoints (from `metric_points`)

These endpoints enable a dashboard to query a curated set of series.

- `GET /api/series?server_id=<id>`
  - Lists available `(measurement, field)` pairs for that server.
  - Query params: `page`, `page_size` (default `25`, max `200`)

- `GET /api/series/latest?server_id=<id>&measurement=<m>&field=<f>&tags=<json>`
  - Returns the latest point for a series.
  - `tags` is optional JSON used for filtering via Postgres JSONB `@>`.

- `GET /api/series/query?server_id=<id>&measurement=<m>&field=<f>&range=<interval>&tags=<json>`
  - Returns time-ordered points for a series in the requested range.
  - `tags` is optional JSON.
  - Supports `page`, `page_size` pagination on the result set.

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

## Rate limiting

An IP-based sliding-window limiter wraps every HTTP handler. Configure via:

- `RATE_LIMIT_WINDOW_SECONDS` (default `60`)
- `RATE_LIMIT_MAX` (default `120`)

Set either to `0` to disable the limiter.

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
