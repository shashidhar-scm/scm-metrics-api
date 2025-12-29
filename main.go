package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	dbpkg "metrics-api/internal/db"
	"metrics-api/internal/handlers"
	"metrics-api/internal/models"
	"metrics-api/internal/repository"
	"metrics-api/internal/routes"

	_ "github.com/lib/pq"
)

var (
	db               *sql.DB
	metricPointsChan chan models.SeriesPoint
	limiter          *rateLimiter
	metricsRepo      *repository.MetricsRepository
)

const (
	batchSize        = 1000 // flush immediately when queue grows
	flushIntervalSec = 1    // flush at least every 1 second
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

type rateLimiter struct {
	window  time.Duration
	max     int
	mu      sync.Mutex
	clients map[string]*rateEntry
}

type rateEntry struct {
	count   int
	expires time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	if max <= 0 || window <= 0 {
		return nil
	}
	return &rateLimiter{
		window:  window,
		max:     max,
		clients: make(map[string]*rateEntry),
	}
}

func (l *rateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.clients[key]
	if !ok || now.After(entry.expires) {
		l.clients[key] = &rateEntry{count: 1, expires: now.Add(l.window)}
		return true
	}

	if entry.count >= l.max {
		return false
	}

	entry.count++
	return true
}

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if limiter == nil {
			next(w, r)
			return
		}

		if !limiter.Allow(clientKey(r)) {
			handlers.WriteJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next(w, r)
	}
}

func clientKey(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func metricWriter() {
	ticker := time.NewTicker(time.Duration(flushIntervalSec) * time.Second)
	defer ticker.Stop()

	buffer := make([]models.SeriesPoint, 0, batchSize)

	for {
		select {
		case p := <-metricPointsChan:
			buffer = append(buffer, p)
			if len(buffer) >= batchSize {
				flushBatch(buffer)
				buffer = buffer[:0]
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				flushBatch(buffer)
				buffer = buffer[:0]
			}
		}
	}
}

func flushBatch(batch []models.SeriesPoint) {
	if len(batch) == 0 || metricsRepo == nil {
		return
	}

	if err := metricsRepo.SaveSeriesPoints(context.Background(), batch); err != nil {
		log.Println("batch: insert err:", err)
	}
}

func runMigrations(db *sql.DB) error {
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
        filename TEXT PRIMARY KEY,
        applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`)

	files, err := filepath.Glob("./migrations/**/*.sql")
	if err != nil {
		return err
	}

	for _, file := range files {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM _migrations WHERE filename=$1)`, file).Scan(&exists)
		if err != nil {
			return fmt.Errorf("migration check failed on %s: %w", file, err)
		}
		if exists {
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		log.Println("Running migration:", file)
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("migration %s failed: %w", file, err)
		}

		_, _ = db.Exec(`INSERT INTO _migrations(filename) VALUES($1)`, file)
	}
	return nil
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---------- MAIN ----------

func main() {
	var err error

	db, err = dbpkg.Setup()
	if err != nil {
		log.Fatal("DB setup failed:", err)
	}

	metricsRepo = repository.NewMetricsRepository(db)

	// if err := runMigrations(db); err != nil {       // <-- ADD THIS LINE
	// 	log.Fatal("SQL migrations failed:", err)
	// }

	rateLimitWindow := time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second
	rateLimitMax := getEnvInt("RATE_LIMIT_MAX", 120)
	limiter = newRateLimiter(rateLimitMax, rateLimitWindow)

	debug := getEnv("DEBUG", "") != ""
	handler := handlers.NewMetricsHandler(metricsRepo, metricPointsChan, debug)

	routes.Register(http.DefaultServeMux, rateLimitMiddleware, routes.Handlers{
		Root:              handler.Root,
		Ingest:            handler.Ingest,
		Servers:           handler.Servers,
		ServersStatus:     handler.ServersStatus,
		ServersStatusCity: handler.ServersStatusCity,
		MetricsLatest:     handler.Latest,
		MetricsHistory:    handler.History,
		SeriesList:        handler.SeriesList,
		SeriesLatest:      handler.SeriesLatest,
		SeriesQuery:       handler.SeriesQuery,
	})

	metricPointsChan = make(chan models.SeriesPoint, 5000)
	go metricWriter()
	log.Println("Metrics API listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", withCORS(http.DefaultServeMux)))
}
