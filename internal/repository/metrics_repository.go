package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"metrics-api/internal/models"
)

// MetricsRepository provides DB operations for metrics ingestion and querying.
type MetricsRepository struct {
	db *sql.DB
}

func NewMetricsRepository(db *sql.DB) *MetricsRepository {
	return &MetricsRepository{db: db}
}

func (r *MetricsRepository) SaveMetric(ctx context.Context, m models.CleanMetric) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO server_metrics(time, server_id, cpu, memory, temperature, sound_volume_percent, memory_total_bytes, memory_used_bytes, disk, disk_total_bytes, disk_used_bytes, disk_free_bytes, net_bytes_sent, net_bytes_recv, net_daily_rx_bytes, net_daily_tx_bytes, net_monthly_rx_bytes, net_monthly_tx_bytes, uptime, city, city_name, region, region_name)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
         ON CONFLICT (server_id, time) DO UPDATE
         SET cpu = EXCLUDED.cpu,
             memory = EXCLUDED.memory,
             temperature = EXCLUDED.temperature,
             sound_volume_percent = EXCLUDED.sound_volume_percent,
             memory_total_bytes = EXCLUDED.memory_total_bytes,
             memory_used_bytes = EXCLUDED.memory_used_bytes,
             disk = EXCLUDED.disk,
             disk_total_bytes = EXCLUDED.disk_total_bytes,
             disk_used_bytes = EXCLUDED.disk_used_bytes,
             disk_free_bytes = EXCLUDED.disk_free_bytes,
             net_bytes_sent = EXCLUDED.net_bytes_sent,
             net_bytes_recv = EXCLUDED.net_bytes_recv,
             net_daily_rx_bytes = EXCLUDED.net_daily_rx_bytes,
             net_daily_tx_bytes = EXCLUDED.net_daily_tx_bytes,
             net_monthly_rx_bytes = EXCLUDED.net_monthly_rx_bytes,
             net_monthly_tx_bytes = EXCLUDED.net_monthly_tx_bytes,
             uptime = EXCLUDED.uptime,
             city = EXCLUDED.city,
             city_name = EXCLUDED.city_name,
             region = EXCLUDED.region,
             region_name = EXCLUDED.region_name`,
		m.Time, m.ServerID, m.CPU, m.Memory, m.Temperature, m.SoundVolumePercent, m.MemoryTotalBytes, m.MemoryUsedBytes, m.Disk, m.DiskTotalBytes, m.DiskUsedBytes, m.DiskFreeBytes, m.NetBytesSent, m.NetBytesRecv, m.NetDailyRxBytes, m.NetDailyTxBytes, m.NetMonthlyRxBytes, m.NetMonthlyTxBytes, m.Uptime, m.City, m.CityName, m.Region, m.RegionName,
	)
	return err
}

func (r *MetricsRepository) SaveSeriesPoints(ctx context.Context, points []models.SeriesPoint) error {
	if len(points) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO metric_points(time, server_id, measurement, field, value_double, value_int, tags) VALUES ($1, $2, $3, $4, $5, $6, $7)`)
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
		if _, err := stmt.ExecContext(ctx, p.Time, p.ServerID, p.Measurement, p.Field, p.ValueDouble, p.ValueInt, p.TagsJSON); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *MetricsRepository) ListSeriesMeta(ctx context.Context, serverID string, limit, offset int) ([]models.SeriesMeta, bool, error) {
	limitPlusOne := limit + 1
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT measurement, field
         FROM metric_points
         WHERE server_id = $1
         ORDER BY measurement, field
         LIMIT $2 OFFSET $3`,
		serverID, limitPlusOne, offset,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var out []models.SeriesMeta
	for rows.Next() {
		var m models.SeriesMeta
		if err := rows.Scan(&m.Measurement, &m.Field); err != nil {
			return nil, false, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(out) > limit {
		hasMore = true
		out = out[:limit]
	}
	return out, hasMore, nil
}

func (r *MetricsRepository) SeriesLatest(ctx context.Context, serverID, measurement, field, tagFilter string) (*models.SeriesPointResponse, error) {
	var resp models.SeriesPointResponse
	var tagsRaw []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT time, server_id, measurement, field, value_double, value_int, tags
         FROM metric_points
         WHERE server_id = $1 AND measurement = $2 AND field = $3 AND tags @> $4::jsonb
         ORDER BY time DESC
         LIMIT 1`,
		serverID, measurement, field, tagFilter,
	).Scan(&resp.Time, &resp.ServerID, &resp.Measurement, &resp.Field, &resp.ValueDouble, &resp.ValueInt, &tagsRaw)
	if err != nil {
		return nil, err
	}
	var tags map[string]interface{}
	_ = json.Unmarshal(tagsRaw, &tags)
	resp.Tags = tags
	return &resp, nil
}

func (r *MetricsRepository) SeriesQuery(ctx context.Context, serverID, measurement, field, rng, tagFilter string, limit, offset int) ([]models.SeriesPointResponse, bool, error) {
	limitPlusOne := limit + 1
	rows, err := r.db.QueryContext(ctx,
		`SELECT time, server_id, measurement, field, value_double, value_int, tags
         FROM metric_points
         WHERE server_id = $1 AND measurement = $2 AND field = $3
           AND time > now() - $4::interval AND tags @> $5::jsonb
         ORDER BY time
         LIMIT $6 OFFSET $7`,
		serverID, measurement, field, rng, tagFilter, limitPlusOne, offset,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var out []models.SeriesPointResponse
	for rows.Next() {
		var resp models.SeriesPointResponse
		var tagsRaw []byte
		if err := rows.Scan(&resp.Time, &resp.ServerID, &resp.Measurement, &resp.Field, &resp.ValueDouble, &resp.ValueInt, &tagsRaw); err != nil {
			return nil, false, err
		}
		var tags map[string]interface{}
		_ = json.Unmarshal(tagsRaw, &tags)
		resp.Tags = tags
		out = append(out, resp)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(out) > limit {
		hasMore = true
		out = out[:limit]
	}
	return out, hasMore, nil
}

func (r *MetricsRepository) buildServerFilter(city, region string) (string, []interface{}) {
	args := make([]interface{}, 0, 2)
	parts := make([]string, 0, 2)
	if city != "" {
		args = append(args, city)
		parts = append(parts, fmt.Sprintf("city = $%d", len(args)))
	}
	if region != "" {
		args = append(args, region)
		parts = append(parts, fmt.Sprintf("region = $%d", len(args)))
	}

	if len(parts) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func (r *MetricsRepository) Servers(ctx context.Context, city, region string, limit, offset int) ([]string, bool, error) {
	whereClause, args := r.buildServerFilter(city, region)
	limitPlusOne := limit + 1
	args = append(args, limitPlusOne, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT server_id FROM server_metrics`+whereClause+fmt.Sprintf(" ORDER BY server_id LIMIT $%d OFFSET $%d", len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var servers []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, false, err
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(servers) > limit {
		hasMore = true
		servers = servers[:limit]
	}
	return servers, hasMore, nil
}

func (r *MetricsRepository) ServerStatus(ctx context.Context, city, region string, limit, offset int) ([]models.ServerStatus, bool, error) {
	whereClause, args := r.buildServerFilter(city, region)
	limitPlusOne := limit + 1
	args = append(args, limitPlusOne, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT ON (server_id)
            server_id, time, city, city_name, region, region_name
         FROM server_metrics`+whereClause+fmt.Sprintf(" ORDER BY server_id, time DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var out []models.ServerStatus
	for rows.Next() {
		var s models.ServerStatus
		if err := rows.Scan(&s.ServerID, &s.LastSeen, &s.City, &s.CityName, &s.Region, &s.RegionName); err != nil {
			return nil, false, err
		}
		age := now.Sub(s.LastSeen)
		s.AgeSeconds = int64(age.Seconds())
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(out) > limit {
		hasMore = true
		out = out[:limit]
	}
	return out, hasMore, nil
}

func (r *MetricsRepository) CityStatusSummary(ctx context.Context, region, threshold string, limit, offset int) ([]models.CityStatusSummary, bool, error) {
	q := `WITH latest AS (
            SELECT DISTINCT ON (server_id) server_id, time, city, city_name
            FROM server_metrics`
	args := []interface{}{threshold}
	where := ""
	if region != "" {
		args = append(args, region)
		where += " region = $" + fmt.Sprint(len(args))
	}
	if where != "" {
		q += " WHERE" + where
	}
	q += `
            ORDER BY server_id, time DESC
        )
        SELECT
            COALESCE(city, '') AS city,
            MAX(COALESCE(city_name, '')) AS city_name,
            SUM(CASE WHEN now() - time <= $1::interval THEN 1 ELSE 0 END) AS online,
            SUM(CASE WHEN now() - time >  $1::interval THEN 1 ELSE 0 END) AS offline,
            COUNT(*) AS total
        FROM latest
        GROUP BY 1
        ORDER BY 1`

	limitPlusOne := limit + 1
	argsWithPage := append(append([]interface{}{}, args...), limitPlusOne, offset)
	rows, err := r.db.QueryContext(ctx, q+fmt.Sprintf("\n        LIMIT $%d OFFSET $%d", len(argsWithPage)-1, len(argsWithPage)), argsWithPage...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var out []models.CityStatusSummary
	for rows.Next() {
		var s models.CityStatusSummary
		if err := rows.Scan(&s.City, &s.CityName, &s.OnlineCount, &s.OfflineCount, &s.Total); err != nil {
			return nil, false, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(out) > limit {
		hasMore = true
		out = out[:limit]
	}
	return out, hasMore, nil
}

func (r *MetricsRepository) LatestMetrics(ctx context.Context, limit, offset int) ([]models.LatestMetric, bool, error) {
	limitPlusOne := limit + 1
	rows, err := r.db.QueryContext(ctx, `
        SELECT DISTINCT ON (server_id)
            server_id, time, cpu, memory, temperature, sound_volume_percent,
            memory_total_bytes, memory_used_bytes,
            disk, disk_total_bytes, disk_used_bytes, disk_free_bytes,
            net_bytes_sent, net_bytes_recv,
            net_daily_rx_bytes, net_daily_tx_bytes,
            net_monthly_rx_bytes, net_monthly_tx_bytes,
            uptime, city, city_name, region, region_name
        FROM server_metrics
        ORDER BY server_id, time DESC
        LIMIT $1 OFFSET $2`, limitPlusOne, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var result []models.LatestMetric
	for rows.Next() {
		var m models.LatestMetric
		if err := rows.Scan(
			&m.ServerID,
			&m.Time,
			&m.CPU,
			&m.Memory,
			&m.Temperature,
			&m.SoundVolumePercent,
			&m.MemoryTotalBytes,
			&m.MemoryUsedBytes,
			&m.Disk,
			&m.DiskTotalBytes,
			&m.DiskUsedBytes,
			&m.DiskFreeBytes,
			&m.NetBytesSent,
			&m.NetBytesRecv,
			&m.NetDailyRxBytes,
			&m.NetDailyTxBytes,
			&m.NetMonthlyRxBytes,
			&m.NetMonthlyTxBytes,
			&m.Uptime,
			&m.City,
			&m.CityName,
			&m.Region,
			&m.RegionName,
		); err != nil {
			return nil, false, err
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(result) > limit {
		hasMore = true
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (r *MetricsRepository) HistoryMetrics(ctx context.Context, serverID, rng string, limit, offset int) ([]models.HistoryMetric, bool, error) {
	limitPlusOne := limit + 1
	rows, err := r.db.QueryContext(ctx, `
        SELECT time, cpu, memory, temperature, sound_volume_percent,
               memory_total_bytes, memory_used_bytes,
               disk, disk_total_bytes, disk_used_bytes, disk_free_bytes,
               net_bytes_sent, net_bytes_recv,
               net_daily_rx_bytes, net_daily_tx_bytes,
               net_monthly_rx_bytes, net_monthly_tx_bytes,
               uptime, city, city_name, region, region_name
        FROM server_metrics
        WHERE server_id = $1 AND time > now() - $2::interval
        ORDER BY time
        LIMIT $3 OFFSET $4`, serverID, rng, limitPlusOne, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var result []models.HistoryMetric
	for rows.Next() {
		var m models.HistoryMetric
		if err := rows.Scan(
			&m.Time,
			&m.CPU,
			&m.Memory,
			&m.Temperature,
			&m.SoundVolumePercent,
			&m.MemoryTotalBytes,
			&m.MemoryUsedBytes,
			&m.Disk,
			&m.DiskTotalBytes,
			&m.DiskUsedBytes,
			&m.DiskFreeBytes,
			&m.NetBytesSent,
			&m.NetBytesRecv,
			&m.NetDailyRxBytes,
			&m.NetDailyTxBytes,
			&m.NetMonthlyRxBytes,
			&m.NetMonthlyTxBytes,
			&m.Uptime,
			&m.City,
			&m.CityName,
			&m.Region,
			&m.RegionName,
		); err != nil {
			return nil, false, err
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(result) > limit {
		hasMore = true
		result = result[:limit]
	}
	return result, hasMore, nil
}
