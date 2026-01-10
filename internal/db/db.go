package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Config struct {
	Host         string
	Port         string
	User         string
	Password     string
	Name         string
	SSLMode      string
	AdminDB      string
	URL          string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
	MaxIdleTime  time.Duration
}

func Setup() (*sql.DB, error) {
	cfg := LoadConfig()
	return SetupWithConfig(cfg)
}

func SetupWithConfig(cfg Config) (*sql.DB, error) {
	if err := ensureDatabaseExists(cfg); err != nil {
		return nil, fmt.Errorf("ensure database: %w", err)
	}

	conn, err := sql.Open("postgres", cfg.connectionString())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(cfg.MaxOpenConns)
	conn.SetMaxIdleConns(cfg.MaxIdleConns)
	conn.SetConnMaxLifetime(cfg.MaxLifetime)
	conn.SetConnMaxIdleTime(cfg.MaxIdleTime)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := ensureSchema(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	return conn, nil
}

func LoadConfig() Config {
	return Config{
		Host:         getEnv("PSQL_HOST", "localhost"),
		Port:         getEnv("PSQL_PORT", "5432"),
		User:         getEnv("PSQL_USER", "postgres"),
		Password:     getEnv("PSQL_PASSWORD", "secret"),
		Name:         getEnv("PSQL_DB_NAME", "metrics"),
		SSLMode:      getEnv("PSQL_SSLMODE", "disable"),
		AdminDB:      getEnv("PSQL_ADMIN_DB", "postgres"),
		URL:          os.Getenv("DATABASE_URL"),
		MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 10),
		MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),
		MaxLifetime:  time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,
		MaxIdleTime:  time.Duration(getEnvInt("DB_CONN_MAX_IDLE_MIN", 5)) * time.Minute,
	}
}

func (c Config) connectionString() string {
	if c.URL != "" {
		return c.URL
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

func (c Config) adminConnectionString() string {
	if c.URL != "" {
		u, err := url.Parse(c.URL)
		if err == nil {
			u.Path = "/" + c.AdminDB
			return u.String()
		}
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.AdminDB, c.SSLMode,
	)
}

func ensureDatabaseExists(cfg Config) error {
	adminConn, err := sql.Open("postgres", cfg.adminConnectionString())
	if err != nil {
		return err
	}
	defer adminConn.Close()

	if err := adminConn.Ping(); err != nil {
		return err
	}

	var exists bool
	if err := adminConn.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.Name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	dbName := strings.ReplaceAll(cfg.Name, "\"", "\"\"")
	owner := strings.ReplaceAll(cfg.User, "\"", "\"\"")
	_, err = adminConn.Exec(fmt.Sprintf("CREATE DATABASE \"%s\" OWNER \"%s\"", dbName, owner))
	return err
}

func ensureSchema(conn *sql.DB) error {
	if _, err := conn.Exec("CREATE TABLE IF NOT EXISTS server_metrics (time TIMESTAMPTZ NOT NULL, server_id TEXT NOT NULL, cpu DOUBLE PRECISION NOT NULL DEFAULT 0, memory DOUBLE PRECISION NOT NULL DEFAULT 0, temperature DOUBLE PRECISION NOT NULL DEFAULT 0, chassis_temperature DOUBLE PRECISION NOT NULL DEFAULT 0, hotspot_temperature DOUBLE PRECISION NOT NULL DEFAULT 0, power_online BOOLEAN NOT NULL DEFAULT FALSE, battery_present BOOLEAN NOT NULL DEFAULT FALSE, battery_charge_pct BIGINT NOT NULL DEFAULT 0, battery_voltage_mv BIGINT NOT NULL DEFAULT 0, battery_current_ma BIGINT NOT NULL DEFAULT 0, sound_volume_percent BIGINT NOT NULL DEFAULT 0, sound_muted BOOLEAN NOT NULL DEFAULT FALSE, display_connected BOOLEAN NOT NULL DEFAULT FALSE, display_width BIGINT NOT NULL DEFAULT 0, display_height BIGINT NOT NULL DEFAULT 0, display_refresh_hz BIGINT NOT NULL DEFAULT 0, display_primary BOOLEAN NOT NULL DEFAULT FALSE, display_dpms_enabled BOOLEAN NOT NULL DEFAULT FALSE, fan_rpm BIGINT NOT NULL DEFAULT 0, memory_total_bytes BIGINT NOT NULL DEFAULT 0, memory_used_bytes BIGINT NOT NULL DEFAULT 0, disk DOUBLE PRECISION NOT NULL DEFAULT 0, disk_total_bytes BIGINT NOT NULL DEFAULT 0, disk_used_bytes BIGINT NOT NULL DEFAULT 0, disk_free_bytes BIGINT NOT NULL DEFAULT 0, net_bytes_sent BIGINT NOT NULL DEFAULT 0, net_bytes_recv BIGINT NOT NULL DEFAULT 0, net_daily_rx_bytes BIGINT NOT NULL DEFAULT 0, net_daily_tx_bytes BIGINT NOT NULL DEFAULT 0, net_monthly_rx_bytes BIGINT NOT NULL DEFAULT 0, net_monthly_tx_bytes BIGINT NOT NULL DEFAULT 0, input_devices_healthy BIGINT NOT NULL DEFAULT 0, input_devices_missing BIGINT NOT NULL DEFAULT 0, input_devices JSONB, link_interface TEXT NOT NULL DEFAULT '', link_up BOOLEAN NOT NULL DEFAULT FALSE, link_speed_mbps BIGINT NOT NULL DEFAULT 0, link_duplex_full BOOLEAN NOT NULL DEFAULT FALSE, link_autoneg BOOLEAN NOT NULL DEFAULT FALSE, link_rx_errors BIGINT NOT NULL DEFAULT 0, link_tx_errors BIGINT NOT NULL DEFAULT 0, link_rx_dropped BIGINT NOT NULL DEFAULT 0, link_tx_dropped BIGINT NOT NULL DEFAULT 0, link_signal_dbm BIGINT NOT NULL DEFAULT 0, link_tx_bitrate_mbps BIGINT NOT NULL DEFAULT 0, link_rx_bitrate_mbps BIGINT NOT NULL DEFAULT 0, link_state JSONB, process_statuses JSONB, uptime BIGINT NOT NULL DEFAULT 0, city TEXT, city_name TEXT, region TEXT, region_name TEXT)"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS sound_volume_percent BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS sound_muted BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS chassis_temperature DOUBLE PRECISION NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS hotspot_temperature DOUBLE PRECISION NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS power_online BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS battery_present BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS battery_charge_pct BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS battery_voltage_mv BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS battery_current_ma BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS display_connected BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS display_width BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS display_height BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS display_refresh_hz BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS display_primary BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS display_dpms_enabled BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS fan_rpm BIGINT NOT NULL DEFAULT 0"); err != nil {
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
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS temperature DOUBLE PRECISION NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS net_bytes_sent BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS net_bytes_recv BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS net_daily_rx_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS net_daily_tx_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS net_monthly_rx_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS net_monthly_tx_bytes BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS input_devices_healthy BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS input_devices_missing BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS input_devices JSONB"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_interface TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_up BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_speed_mbps BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_duplex_full BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_autoneg BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_rx_errors BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_tx_errors BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_rx_dropped BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_tx_dropped BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS link_state JSONB"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS process_statuses JSONB"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS uptime BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS city TEXT"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS city_name TEXT"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS region TEXT"); err != nil {
		return err
	}
	if _, err := conn.Exec("ALTER TABLE server_metrics ADD COLUMN IF NOT EXISTS region_name TEXT"); err != nil {
		return err
	}
	if _, err := conn.Exec("CREATE INDEX IF NOT EXISTS idx_server_metrics_server_id_time_desc ON server_metrics (server_id, time DESC)"); err != nil {
		return err
	}
	if _, err := conn.Exec("CREATE INDEX IF NOT EXISTS idx_server_metrics_time ON server_metrics (time DESC)"); err != nil {
		return err
	}
	if _, err := conn.Exec("CREATE INDEX IF NOT EXISTS idx_server_metrics_region_server_id_time_desc ON server_metrics (region, server_id, time DESC)"); err != nil {
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
	if timescaleAvailable {
		if _, err := conn.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb"); err != nil {
			return err
		}
		if _, err := conn.Exec("SELECT create_hypertable('server_metrics', 'time', if_not_exists => TRUE)"); err != nil {
			return err
		}
		if _, err := conn.Exec("SELECT create_hypertable('metric_points', 'time', if_not_exists => TRUE)"); err != nil {
			return err
		}
	}

	return nil
}

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
