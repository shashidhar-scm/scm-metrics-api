package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"metrics-api/internal/models"
	"metrics-api/internal/repository"
)

type MetricsHandler struct {
	repo           *repository.MetricsRepository
	metricPoints   chan models.SeriesPoint
	debugLoggingOn bool
}

func NewMetricsHandler(repo *repository.MetricsRepository, metricPoints chan models.SeriesPoint, debug bool) *MetricsHandler {
	return &MetricsHandler{
		repo:           repo,
		metricPoints:   metricPoints,
		debugLoggingOn: debug,
	}
}

func (h *MetricsHandler) Root(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MetricsHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var payload models.TelegrafPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.debugLoggingOn {
		log.Printf("ingest: received %d metrics", len(payload.Metrics))
	}

	var cm models.CleanMetric
	var points []models.SeriesPoint

	var diskTotalBytes int64
	var diskUsedBytes int64
	var diskFreeBytes int64
	seenDisk := make(map[string]struct{})

	for _, m := range payload.Metrics {
		if h.debugLoggingOn {
			log.Printf("ingest: metric name=%s tags=%v fields=%v", m.Name, m.Tags, keysOf(m.Fields))
		}

		if cm.ServerID == "" {
			cm.ServerID = m.Tags["server_id"]
			if cm.ServerID == "" || cm.ServerID == "$HOSTNAME" {
				cm.ServerID = m.Tags["host"]
			}
		}

		if strings.HasPrefix(m.Name, "kiosk_") {
			if cm.City == "" && m.Tags["city"] != "" {
				cm.City = m.Tags["city"]
			}
			if cm.CityName == "" && m.Tags["city_full_name"] != "" {
				cm.CityName = m.Tags["city_full_name"]
			}
			if cm.Region == "" && m.Tags["code"] != "" {
				cm.Region = m.Tags["code"]
			}
			if cm.RegionName == "" && m.Tags["name"] != "" {
				cm.RegionName = m.Tags["name"]
			}
		}

		if cm.Time.IsZero() && m.Timestamp > 0 {
			cm.Time = time.Unix(int64(m.Timestamp), 0)
		}
		ptTime := time.Unix(int64(m.Timestamp), 0)

		switch m.Name {
		case "cpu":
			if m.Tags["cpu"] != "cpu-total" {
				continue
			}
			if cm.CPU == 0 {
				if v, ok := m.Fields["usage_idle"].(float64); ok {
					cm.CPU = 100 - v
				}
			}

			points = append(points,
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_user", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_system", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_iowait", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "cpu", "usage_steal", m.Fields, m.Tags),
			)

		case "mem":
			if v, ok := m.Fields["available_percent"].(float64); ok {
				cm.Memory = 100 - v
			}
			if v, ok := m.Fields["total"].(float64); ok {
				cm.MemoryTotalBytes = int64(v)
			}
			if v, ok := m.Fields["used"].(float64); ok {
				cm.MemoryUsedBytes = int64(v)
			}

			points = append(points,
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
			switch m.Tags["fstype"] {
			case "tmpfs", "devtmpfs", "overlay", "squashfs",
				"proc", "sysfs", "cgroup", "cgroup2",
				"nsfs", "rpc_pipefs", "devpts",
				"securityfs", "pstore", "hugetlbfs",
				"mqueue", "tracefs", "fusectl":
				continue
			}

			key := m.Tags["device"] + "|" + m.Tags["path"]
			if _, ok := seenDisk[key]; ok {
				continue
			}
			seenDisk[key] = struct{}{}

			if v, ok := m.Fields["total"].(float64); ok {
				diskTotalBytes += int64(v)
			}
			if v, ok := m.Fields["used"].(float64); ok {
				diskUsedBytes += int64(v)
			}
			if v, ok := m.Fields["free"].(float64); ok {
				diskFreeBytes += int64(v)
			}

		case "diskio":
			points = append(points,
				seriesPointInt(ptTime, cm.ServerID, "diskio", "read_bytes", m.Fields, m.Tags),
				seriesPointInt(ptTime, cm.ServerID, "diskio", "write_bytes", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "diskio", "io_util", m.Fields, m.Tags),
				seriesPointFloat(ptTime, cm.ServerID, "diskio", "io_await", m.Fields, m.Tags),
			)

		case "system":
			if v, ok := m.Fields["uptime"].(float64); ok {
				cm.Uptime = int64(v)
				points = append(points,
					seriesPointInt(ptTime, cm.ServerID, "system", "uptime", m.Fields, m.Tags),
				)
			}

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

	if diskTotalBytes > 0 {
		cm.Disk = float64(diskUsedBytes) * 100 / float64(diskTotalBytes)
		cm.DiskTotalBytes = diskTotalBytes
		cm.DiskUsedBytes = diskUsedBytes
		cm.DiskFreeBytes = diskFreeBytes
	}
	points = append(points,
		models.SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "total", ValueInt: &cm.DiskTotalBytes, TagsJSON: []byte(`{"aggregated":true}`)},
		models.SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "used", ValueInt: &cm.DiskUsedBytes, TagsJSON: []byte(`{"aggregated":true}`)},
		models.SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "free", ValueInt: &cm.DiskFreeBytes, TagsJSON: []byte(`{"aggregated":true}`)},
		models.SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "disk", Field: "used_percent", ValueDouble: &cm.Disk, TagsJSON: []byte(`{"aggregated":true}`)},
	)

	if h.debugLoggingOn {
		log.Printf("ingest: parsed server_id=%s time=%s cpu=%.4f memory=%.4f memory_total_bytes=%d memory_used_bytes=%d disk=%.4f disk_total_bytes=%d disk_used_bytes=%d disk_free_bytes=%d",
			cm.ServerID, cm.Time.UTC().Format(time.RFC3339), cm.CPU, cm.Memory,
			cm.MemoryTotalBytes, cm.MemoryUsedBytes, cm.Disk, cm.DiskTotalBytes, cm.DiskUsedBytes, cm.DiskFreeBytes)
	}

	if err := h.repo.SaveMetric(r.Context(), cm); err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "failed to persist metric: "+err.Error())
		return
	}

	for _, p := range points {
		if p.ServerID == "" {
			continue
		}
		h.metricPoints <- p
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MetricsHandler) SeriesList(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	if serverID == "" {
		WriteJSONError(w, http.StatusBadRequest, "server_id required")
		return
	}

	items, err := h.repo.ListSeriesMeta(r.Context(), serverID)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, items)
}

func (h *MetricsHandler) SeriesLatest(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	measurement := r.URL.Query().Get("measurement")
	field := r.URL.Query().Get("field")
	if serverID == "" || measurement == "" || field == "" {
		WriteJSONError(w, http.StatusBadRequest, "server_id, measurement, field required")
		return
	}

	tagFilter := r.URL.Query().Get("tags")
	if tagFilter == "" {
		tagFilter = "{}"
	}

	resp, err := h.repo.SeriesLatest(r.Context(), serverID, measurement, field, tagFilter)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (h *MetricsHandler) SeriesQuery(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	measurement := r.URL.Query().Get("measurement")
	field := r.URL.Query().Get("field")
	if serverID == "" || measurement == "" || field == "" {
		WriteJSONError(w, http.StatusBadRequest, "server_id, measurement, field required")
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

	out, err := h.repo.SeriesQuery(r.Context(), serverID, measurement, field, rng, tagFilter)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, out)
}

func (h *MetricsHandler) Servers(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	region := r.URL.Query().Get("region")

	servers, err := h.repo.Servers(r.Context(), city, region)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, servers)
}

func (h *MetricsHandler) ServersStatus(w http.ResponseWriter, r *http.Request) {
	thresholdStr := r.URL.Query().Get("threshold")
	if thresholdStr == "" {
		thresholdStr = "5m"
	}
	threshold, err := time.ParseDuration(thresholdStr)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid threshold")
		return
	}

	city := r.URL.Query().Get("city")
	region := r.URL.Query().Get("region")

	statuses, err := h.repo.ServerStatus(r.Context(), city, region)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range statuses {
		age := time.Duration(statuses[i].AgeSeconds) * time.Second
		statuses[i].Online = age <= threshold
	}

	WriteJSON(w, http.StatusOK, statuses)
}

func (h *MetricsHandler) ServersStatusCity(w http.ResponseWriter, r *http.Request) {
	thresholdStr := r.URL.Query().Get("threshold")
	if thresholdStr == "" {
		thresholdStr = "5m"
	}
	if _, err := time.ParseDuration(thresholdStr); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid threshold")
		return
	}

	region := r.URL.Query().Get("region")

	items, err := h.repo.CityStatusSummary(r.Context(), region, thresholdStr)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, items)
}

func (h *MetricsHandler) Latest(w http.ResponseWriter, r *http.Request) {
	result, err := h.repo.LatestMetrics(r.Context())
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

func (h *MetricsHandler) History(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	if serverID == "" {
		WriteJSONError(w, http.StatusBadRequest, "server_id required")
		return
	}

	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "1h"
	}

	result, err := h.repo.HistoryMetrics(r.Context(), serverID, rng)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

func WriteJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func seriesPointFloat(t time.Time, serverID, measurement, field string, fields map[string]interface{}, tags map[string]string) models.SeriesPoint {
	var vPtr *float64
	if v, ok := fields[field].(float64); ok {
		vv := v
		vPtr = &vv
	}
	jb, _ := json.Marshal(tags)
	return models.SeriesPoint{Time: t, ServerID: serverID, Measurement: measurement, Field: field, ValueDouble: vPtr, TagsJSON: jb}
}

func seriesPointInt(t time.Time, serverID, measurement, field string, fields map[string]interface{}, tags map[string]string) models.SeriesPoint {
	var vPtr *int64
	if v, ok := fields[field].(float64); ok {
		vv := int64(v)
		vPtr = &vv
	}
	jb, _ := json.Marshal(tags)
	return models.SeriesPoint{Time: t, ServerID: serverID, Measurement: measurement, Field: field, ValueInt: vPtr, TagsJSON: jb}
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
