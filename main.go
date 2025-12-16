package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}

	host, port, user, pass, name, sslMode := dbConfig()

	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + name + "?sslmode=" + sslMode
}

func dbConfig() (host, port, user, pass, name, sslMode string) {
	host = getEnv("PSQL_HOST", "localhost")
	port = getEnv("PSQL_PORT", "5432")
	user = getEnv("PSQL_USER", "postgres")
	pass = getEnv("PSQL_PASSWORD", "secret")
	name = getEnv("PSQL_DB_NAME", "metrics")
	sslMode = getEnv("PSQL_SSLMODE", "disable")
	return
}

func adminDatabaseURL() string {
	host, port, user, pass, _, sslMode := dbConfig()
	adminDB := getEnv("PSQL_ADMIN_DB", "postgres")
	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + adminDB + "?sslmode=" + sslMode
}

func ensureDatabaseExists() error {
	_, _, user, _, dbName, _ := dbConfig()

	adminDB, err := sql.Open("postgres", adminDatabaseURL())
	if err != nil {
		return err
	}
	defer adminDB.Close()

	if err := adminDB.Ping(); err != nil {
		return err
	}

	var exists bool
	if err := adminDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	identifier := strings.ReplaceAll(dbName, "\"", "\"\"")
	_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE \"%s\" OWNER \"%s\"", identifier, strings.ReplaceAll(user, "\"", "\"\"")))
	return err
}

func ensureSchema(conn *sql.DB) error {
	if _, err := conn.Exec("CREATE TABLE IF NOT EXISTS server_metrics (time TIMESTAMPTZ NOT NULL, server_id TEXT NOT NULL, cpu DOUBLE PRECISION NOT NULL DEFAULT 0, memory DOUBLE PRECISION NOT NULL DEFAULT 0, memory_total_bytes BIGINT NOT NULL DEFAULT 0, memory_used_bytes BIGINT NOT NULL DEFAULT 0, disk DOUBLE PRECISION NOT NULL DEFAULT 0, disk_total_bytes BIGINT NOT NULL DEFAULT 0, disk_used_bytes BIGINT NOT NULL DEFAULT 0, disk_free_bytes BIGINT NOT NULL DEFAULT 0)"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS memory_total_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS memory_used_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS disk_total_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS disk_used_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS disk_free_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("CREATE INDEX IF NOT EXISTS idx_server_metrics_server_id_time_desc ON server_metrics (server_id, time DESC)"); err != nil {
		return err
	}
	if _, err := conn.Exec("CREATE INDEX IF NOT EXISTS idx_server_metrics_time ON server_metrics (time DESC)"); err != nil {
		return err
	}

	if _, err := conn.Exec("CREATE TABLE IF NOT EXISTS metric_points (time TIMESTAMPTZ NOT NULL, server_id TEXT NOT NULL, measurement TEXT NOT NULL, field TEXT NOT NULL, value_double DOUBLE PRECISION NULL, value_int BIGINT NULL, tags JSONB NOT NULL DEFAULT '{}'::jsonb)"); err != nil {
		return err
	}
	if _, err := conn.Exec("CREATE INDEX IF NOT EXISTS idx_metric_points_series_time_desc ON metric_points (server_id, measurement, field, time DESC)"); err != nil {
		return err
	}

	var timescaleAvailable bool
	if err := conn.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb')").Scan(&timescaleAvailable); err != nil {
		return err
	}
	if !timescaleAvailable {
		return nil
	}

	if _, err := conn.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb"); err != nil {
		return err
	}
	if _, err := conn.Exec("SELECT create_hypertable('server_metrics', 'time', if_not_exists => TRUE)"); err != nil {
		return err
	}
	if _, err := conn.Exec("SELECT create_hypertable('metric_points', 'time', if_not_exists => TRUE)"); err != nil {
		return err
	}

	return nil
}

// ---------- INGEST STRUCTS ----------

type TelegrafPayload struct {
	Metrics []Metric `json:"metrics"`
}

type Metric struct {
	Name      string                 `json:"name"`
	Tags      map[string]string      `json:"tags"`
	Fields    map[string]interface{} `json:"fields"`
	Timestamp float64                `json:"timestamp"`
}

type CleanMetric struct {
	ServerID string
	CPU      float64
	Memory   float64
	MemoryTotalBytes int64
	MemoryUsedBytes  int64
	Disk     float64
	DiskTotalBytes int64
	DiskUsedBytes  int64
	DiskFreeBytes  int64
	Time     time.Time
}

// ---------- READ STRUCTS ----------

type LatestMetric struct {
	ServerID string    `json:"server_id"`
	Time     time.Time `json:"time"`
	CPU      float64   `json:"cpu"`
	Memory   float64   `json:"memory"`
	MemoryTotalBytes int64 `json:"memory_total_bytes"`
	MemoryUsedBytes  int64 `json:"memory_used_bytes"`
	Disk     float64   `json:"disk"`
	DiskTotalBytes int64 `json:"disk_total_bytes"`
	DiskUsedBytes  int64 `json:"disk_used_bytes"`
	DiskFreeBytes  int64 `json:"disk_free_bytes"`
}

type HistoryMetric struct {
	Time          time.Time `json:"time"`
	CPU           float64   `json:"cpu"`
	Memory        float64   `json:"memory"`
	MemoryTotalBytes int64  `json:"memory_total_bytes"`
	MemoryUsedBytes  int64  `json:"memory_used_bytes"`
	Disk          float64   `json:"disk"`
	DiskTotalBytes int64    `json:"disk_total_bytes"`
	DiskUsedBytes  int64    `json:"disk_used_bytes"`
	DiskFreeBytes  int64    `json:"disk_free_bytes"`
}

type SeriesMeta struct {
	Measurement string `json:"measurement"`
	Field       string `json:"field"`
}

type SeriesPointResponse struct {
	Time        time.Time                `json:"time"`
	ServerID    string                   `json:"server_id"`
	Measurement string                   `json:"measurement"`
	Field       string                   `json:"field"`
	ValueDouble *float64                 `json:"value_double,omitempty"`
	ValueInt    *int64                   `json:"value_int,omitempty"`
	Tags        map[string]interface{}   `json:"tags"`
}

// ---------- INGEST HANDLER ----------

func ingestHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	debug := getEnv("DEBUG", "") != ""

	var payload TelegrafPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if debug {
		log.Printf("ingest: received %d metrics", len(payload.Metrics))
	}

	var cm CleanMetric
	var points []SeriesPoint

	var diskTotalBytes int64
	var diskUsedBytes int64
	var diskFreeBytes int64
	seenDisk := make(map[string]struct{})

	for _, m := range payload.Metrics {
		if debug {
			log.Printf("ingest: metric name=%s tags=%v fields=%v", m.Name, m.Tags, keysOf(m.Fields))
		}

		if cm.ServerID == "" {
			cm.ServerID = m.Tags["server_id"]
			// 🚨 SAFETY CHECK
			if cm.ServerID == "" || cm.ServerID == "$HOSTNAME" {
				cm.ServerID = m.Tags["host"]
			}
		}

		if cm.Time.IsZero() {
			cm.Time = time.Unix(int64(m.Timestamp), 0)
		}

		ptTime := time.Unix(int64(m.Timestamp), 0)

		switch m.Name {
		case "cpu":
			if cpuTag := m.Tags["cpu"]; cpuTag != "" && cpuTag != "cpu-total" {
				continue
			}
			if v, ok := m.Fields["usage_idle"].(float64); ok {
				cm.CPU = 100 - v
			} else if debug {
				log.Printf("ingest: cpu missing field usage_idle")
			}
			points = append(points,
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_user", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_system", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_iowait", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_steal", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_idle", m.Fields, m.Tags),
			)
		case "mem":
			if v, ok := m.Fields["available_percent"].(float64); ok {
				cm.Memory = 100 - v
			} else if debug {
				log.Printf("ingest: mem missing field available_percent")
			}
			if v, ok := m.Fields["total"].(float64); ok {
				cm.MemoryTotalBytes = int64(v)
			} else if debug {
				log.Printf("ingest: mem missing field total")
			}
			if v, ok := m.Fields["used"].(float64); ok {
				cm.MemoryUsedBytes = int64(v)
			} else if debug {
				log.Printf("ingest: mem missing field used")
			}
			points = append(points,
				seriesPointFloat(ptTime, cm.ServerID, "mem", "available_percent", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "mem", "used_percent", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "mem", "total", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "mem", "used", m.Fields, m.Tags),
			)
		case "swap":
			points = append(points,
				seriesPointFloat(ptTime, cm.ServerID, "swap", "used_percent", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "swap", "in", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "swap", "out", m.Fields, m.Tags),
			)
		case "disk":
			fstype := m.Tags["fstype"]
			switch fstype {
			case "tmpfs", "devtmpfs", "overlay", "squashfs", "proc", "sysfs", "cgroup", "cgroup2", "nsfs", "rpc_pipefs", "devpts", "securityfs", "pstore", "hugetlbfs", "mqueue", "tracefs", "fusectl":
				continue
			}

			path := m.Tags["path"]
			device := m.Tags["device"]
			seenKey := device + "|" + path
			if _, ok := seenDisk[seenKey]; ok {
				continue
			}
			seenDisk[seenKey] = struct{}{}

			var total, used, free int64
			if v, ok := m.Fields["total"].(float64); ok {
				total = int64(v)
			} else if debug {
				log.Printf("ingest: disk missing field total")
			}
			if v, ok := m.Fields["used"].(float64); ok {
				used = int64(v)
			} else if debug {
				log.Printf("ingest: disk missing field used")
			}
			if v, ok := m.Fields["free"].(float64); ok {
				free = int64(v)
			} else if debug {
				log.Printf("ingest: disk missing field free")
			}

			diskTotalBytes += total
			diskUsedBytes += used
			diskFreeBytes += free
		case "diskio":
			points = append(points,
				seriesPointInt(ptTime, cm.ServerID, "diskio", "read_bytes", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "diskio", "write_bytes", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "diskio", "io_util", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "diskio", "io_await", m.Fields, m.Tags),
			)
		case "system":
			points = append(points,
				seriesPointFloat(ptTime, cm.ServerID, "system", "load1", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "system", "load5", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "system", "load15", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "system", "uptime", m.Fields, m.Tags),
			)
		case "processes":
			points = append(points,
				seriesPointInt(ptTime, cm.ServerID, "processes", "running", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "processes", "blocked", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "processes", "zombies", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "processes", "total", m.Fields, m.Tags),
			)
		}
	}

	cm.DiskTotalBytes = diskTotalBytes
	cm.DiskUsedBytes = diskUsedBytes
	cm.DiskFreeBytes = diskFreeBytes
	if diskTotalBytes > 0 {
		cm.Disk = (float64(diskUsedBytes) / float64(diskTotalBytes)) * 100
	}
	points = append(points,
		SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "total", ValueInt: &cm.DiskTotalBytes, TagsJSON: []byte(`{"aggregated":true}`)},
		SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "used", ValueInt: &cm.DiskUsedBytes, TagsJSON: []byte(`{"aggregated":true}`)},
		SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "free", ValueInt: &cm.DiskFreeBytes, TagsJSON: []byte(`{"aggregated":true}`)},
		SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "used_percent", ValueDouble: &cm.Disk, TagsJSON: []byte(`{"aggregated":true}`)},
	)

	if debug {
		log.Printf("ingest: parsed server_id=%s time=%s cpu=%.4f memory=%.4f memory_total_bytes=%d memory_used_bytes=%d disk=%.4f disk_total_bytes=%d disk_used_bytes=%d disk_free_bytes=%d", cm.ServerID, cm.Time.UTC().Format(time.RFC3339), cm.CPU, cm.Memory, cm.MemoryTotalBytes, cm.MemoryUsedBytes, cm.Disk, cm.DiskTotalBytes, cm.DiskUsedBytes, cm.DiskFreeBytes)
	}

	saveMetric(cm)
	if err := saveSeriesPoints(points); err != nil {
		log.Println("metric_points insert error:", err)
	}
	w.WriteHeader(http.StatusAccepted)
}

type SeriesPoint struct {
	Time        time.Time
	ServerID    string
	Measurement string
	Field       string
	ValueDouble *float64
	ValueInt    *int64
	TagsJSON    []byte
}

func seriesPointFloat(t time.Time, serverID, measurement, field string, fields map[string]interface{}, tags map[string]string) SeriesPoint {
	var vPtr *float64
	if v, ok := fields[field].(float64); ok {
		vv := v
		vPtr = &vv
	}
	jb, _ := json.Marshal(tags)
	return SeriesPoint{Time: t, ServerID: serverID, Measurement: measurement, Field: field, ValueDouble: vPtr, TagsJSON: jb}
}

func seriesPointInt(t time.Time, serverID, measurement, field string, fields map[string]interface{}, tags map[string]string) SeriesPoint {
	var vPtr *int64
	if v, ok := fields[field].(float64); ok {
		vv := int64(v)
		vPtr = &vv
	}
	jb, _ := json.Marshal(tags)
	return SeriesPoint{Time: t, ServerID: serverID, Measurement: measurement, Field: field, ValueInt: vPtr, TagsJSON: jb}
}

func saveSeriesPoints(points []SeriesPoint) error {
	if len(points) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO metric_points(time, server_id, measurement, field, value_double, value_int, tags) VALUES ($1, $2, $3, $4, $5, $6, $7)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range points {
		if p.ServerID == "" {
			continue
		}
		if p.TagsJSON == nil {
			p.TagsJSON = []byte(`{}`)
		}
		if _, err := stmt.Exec(p.Time, p.ServerID, p.Measurement, p.Field, p.ValueDouble, p.ValueInt, p.TagsJSON); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func seriesListHandler(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	if serverID == "" {
		http.Error(w, "server_id required", 400)
		return
	}

	rows, err := db.Query(`
		SELECT DISTINCT measurement, field
		FROM metric_points
		WHERE server_id = $1
		ORDER BY measurement, field
	`, serverID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var out []SeriesMeta
	for rows.Next() {
		var m SeriesMeta
		rows.Scan(&m.Measurement, &m.Field)
		out = append(out, m)
	}

	json.NewEncoder(w).Encode(out)
}

func seriesLatestHandler(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	measurement := r.URL.Query().Get("measurement")
	field := r.URL.Query().Get("field")
	if serverID == "" || measurement == "" || field == "" {
		http.Error(w, "server_id, measurement, field required", 400)
		return
	}

	tagFilter := r.URL.Query().Get("tags")
	if tagFilter == "" {
		tagFilter = "{}"
	}

	var resp SeriesPointResponse
	var tagsRaw []byte
	err := db.QueryRow(`
		SELECT time, server_id, measurement, field, value_double, value_int, tags
		FROM metric_points
		WHERE server_id = $1
		AND measurement = $2
		AND field = $3
		AND tags @> $4::jsonb
		ORDER BY time DESC
		LIMIT 1
	`, serverID, measurement, field, tagFilter).Scan(&resp.Time, &resp.ServerID, &resp.Measurement, &resp.Field, &resp.ValueDouble, &resp.ValueInt, &tagsRaw)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var tags map[string]interface{}
	_ = json.Unmarshal(tagsRaw, &tags)
	resp.Tags = tags

	json.NewEncoder(w).Encode(resp)
}

func seriesQueryHandler(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	measurement := r.URL.Query().Get("measurement")
	field := r.URL.Query().Get("field")
	if serverID == "" || measurement == "" || field == "" {
		http.Error(w, "server_id, measurement, field required", 400)
		return
	}

	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "1h"
	}

	tagFilter := r.URL.Query().Get("tags")
	if tagFilter == "" {
		tagFilter = "{}"
	}

	rows, err := db.Query(`
		SELECT time, server_id, measurement, field, value_double, value_int, tags
		FROM metric_points
		WHERE server_id = $1
		AND measurement = $2
		AND field = $3
		AND time > now() - $4::interval
		AND tags @> $5::jsonb
		ORDER BY time
	`, serverID, measurement, field, rng, tagFilter)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var out []SeriesPointResponse
	for rows.Next() {
		var resp SeriesPointResponse
		var tagsRaw []byte
		rows.Scan(&resp.Time, &resp.ServerID, &resp.Measurement, &resp.Field, &resp.ValueDouble, &resp.ValueInt, &tagsRaw)
		var tags map[string]interface{}
		_ = json.Unmarshal(tagsRaw, &tags)
		resp.Tags = tags
		out = append(out, resp)
	}

	json.NewEncoder(w).Encode(out)
}

func keysOf(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------- DB WRITE ----------

func saveMetric(m CleanMetric) {
	_, err := db.Exec(
		`INSERT INTO server_metrics(time, server_id, cpu, memory, memory_total_bytes, memory_used_bytes, disk, disk_total_bytes, disk_used_bytes, disk_free_bytes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		m.Time, m.ServerID, m.CPU, m.Memory, m.MemoryTotalBytes, m.MemoryUsedBytes, m.Disk, m.DiskTotalBytes, m.DiskUsedBytes, m.DiskFreeBytes,
	)
	if err != nil {
		log.Println("DB insert error:", err)
	}
}

// ---------- READ HANDLERS ----------

func serversHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT DISTINCT server_id FROM server_metrics ORDER BY server_id`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var servers []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		servers = append(servers, s)
	}

	json.NewEncoder(w).Encode(servers)
}

func latestHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT DISTINCT ON (server_id)
		  server_id, time, cpu, memory, memory_total_bytes, memory_used_bytes, disk, disk_total_bytes, disk_used_bytes, disk_free_bytes
		FROM server_metrics
		ORDER BY server_id, time DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var result []LatestMetric
	for rows.Next() {
		var m LatestMetric
		rows.Scan(&m.ServerID, &m.Time, &m.CPU, &m.Memory, &m.MemoryTotalBytes, &m.MemoryUsedBytes, &m.Disk, &m.DiskTotalBytes, &m.DiskUsedBytes, &m.DiskFreeBytes)
		result = append(result, m)
	}

	json.NewEncoder(w).Encode(result)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Application is up and running"))
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	if serverID == "" {
		http.Error(w, "server_id required", 400)
		return
	}

	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "1h"
	}

	rows, err := db.Query(`
		SELECT time, cpu, memory, memory_total_bytes, memory_used_bytes, disk, disk_total_bytes, disk_used_bytes, disk_free_bytes
		FROM server_metrics
		WHERE server_id = $1
		AND time > now() - $2::interval
		ORDER BY time
	`, serverID, rng)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var result []HistoryMetric
	for rows.Next() {
		var m HistoryMetric
		rows.Scan(&m.Time, &m.CPU, &m.Memory, &m.MemoryTotalBytes, &m.MemoryUsedBytes, &m.Disk, &m.DiskTotalBytes, &m.DiskUsedBytes, &m.DiskFreeBytes)
		result = append(result, m)
	}

	json.NewEncoder(w).Encode(result)
}

// ---------- MAIN ----------

func main() {
	var err error
	if err := ensureDatabaseExists(); err != nil {
		log.Fatal("DB migration failed:", err)
	}

	db, err = sql.Open(
		"postgres",
		databaseURL(),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("DB connection failed:", err)
	}

	if err := ensureSchema(db); err != nil {
		log.Fatal("DB migration failed:", err)
	}

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/api/metrics", ingestHandler)
	http.HandleFunc("/api/servers", serversHandler)
	http.HandleFunc("/api/metrics/latest", latestHandler)
	http.HandleFunc("/api/metrics/history", historyHandler)
	http.HandleFunc("/api/series", seriesListHandler)
	http.HandleFunc("/api/series/latest", seriesLatestHandler)
	http.HandleFunc("/api/series/query", seriesQueryHandler)

	log.Println("Metrics API listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", withCORS(http.DefaultServeMux)))
}
