package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"metrics-api/internal/models"
	"metrics-api/internal/repository"
)

const (
	defaultPageSize = 25
	maxPageSize     = 200
)

type MetricsHandler struct {
	repo           *repository.MetricsRepository
	metricPoints   chan models.SeriesPoint
	debugLoggingOn bool
	directInsert   bool
	logPayload     bool
	debugServerID  string
}

func NewMetricsHandler(repo *repository.MetricsRepository, metricPoints chan models.SeriesPoint, debug bool, directInsert bool, logPayload bool, debugServerID string) *MetricsHandler {
	return &MetricsHandler{
		repo:           repo,
		metricPoints:   metricPoints,
		debugLoggingOn: debug,
		directInsert:   directInsert,
		logPayload:     logPayload,
		debugServerID:  debugServerID,
	}
}

func deriveServerIdentifiers(metrics []models.Metric) (string, string) {
	for _, metric := range metrics {
		serverID := metric.Tags["server_id"]
		host := metric.Tags["host"]
		if serverID == "" || serverID == "$HOSTNAME" {
			serverID = host
		}
		if serverID != "" || host != "" {
			return serverID, host
		}
	}
	return "", ""
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

	payloadServerID, payloadHost := deriveServerIdentifiers(payload.Metrics)

	if h.logPayload && h.shouldLogForServer(payloadServerID, payloadHost) {
		if b, err := json.Marshal(payload); err == nil {
			log.Printf("ingest_payload: %s", string(b))
		} else {
			log.Printf("ingest_payload_error: %v", err)
		}
	}

	var cm models.CleanMetric
	var points []models.SeriesPoint

	var diskTotalBytes int64
	var diskUsedBytes int64
	var diskFreeBytes int64
	var netBytesSent int64
	var netBytesRecv int64
	var temperatureCaptured bool
	var chassisTemperatureCaptured bool
	var fanCaptured bool
	var volumeCaptured bool
	var hotspotCaptured bool
	var powerOnlineCaptured bool
	var batteryCaptured bool
	var displayCaptured bool
	var displayBestRank int
	displayBestRank = -1
	var dailyVnstatCaptured bool
	var monthlyVnstatCaptured bool
	var sawTemperatureMetric bool
	var sawNetMetric bool
	var inputDevices []models.InputDevice
	var inputDevicesHealthy int64
	var inputDevicesMissing int64
	seenDisk := make(map[string]struct{})

	headerLogged := false

	firstNonEmpty := func(values ...string) string {
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
		return ""
	}

	for _, m := range payload.Metrics {

		if cm.ServerID == "" {
			cm.ServerID = m.Tags["server_id"]
			if cm.ServerID == "" || cm.ServerID == "$HOSTNAME" {
				cm.ServerID = m.Tags["host"]
			}
		}

		logThisMetric := h.shouldLogForServer(cm.ServerID, m.Tags["host"])

		if logThisMetric && !headerLogged {
			log.Printf("ingest: received %d metrics", len(payload.Metrics))
			headerLogged = true
		}

		if logThisMetric {
			log.Printf("ingest: metric name=%s tags=%v fields=%v", m.Name, m.Tags, keysOf(m.Fields))
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

		case "kiosk_display":
			isPrimary := false
			if primaryVal, ok := toInt64(m.Fields["primary"]); ok && primaryVal != 0 {
				isPrimary = true
			}

			rank := 0
			if connectedVal, ok := toInt64(m.Fields["connected"]); ok && connectedVal != 0 {
				rank = 1
				if isPrimary {
					rank = 3
				}
			} else if isPrimary {
				rank = 2
			}

			if !displayCaptured || rank > displayBestRank {
				if connectedVal, ok := toInt64(m.Fields["connected"]); ok {
					cm.DisplayConnected = connectedVal != 0
				} else {
					cm.DisplayConnected = false
				}
				if widthVal, ok := toInt64(m.Fields["width"]); ok {
					cm.DisplayWidth = widthVal
				} else {
					cm.DisplayWidth = 0
				}
				if heightVal, ok := toInt64(m.Fields["height"]); ok {
					cm.DisplayHeight = heightVal
				} else {
					cm.DisplayHeight = 0
				}
				if refreshVal, ok := toInt64(m.Fields["refresh_hz"]); ok {
					cm.DisplayRefreshHz = refreshVal
				} else {
					cm.DisplayRefreshHz = 0
				}
				if dpmsVal, ok := toInt64(m.Fields["dpms_enabled"]); ok {
					cm.DisplayDpmsEnabled = dpmsVal != 0
				} else {
					cm.DisplayDpmsEnabled = false
				}
				cm.DisplayPrimary = isPrimary
				displayCaptured = true
				displayBestRank = rank
			}

			if len(m.Fields) > 0 {
				for fieldName, raw := range m.Fields {
					if iv, ok := toInt64(raw); ok {
						val := iv
						points = append(points, models.SeriesPoint{
							Time:        ptTime,
							ServerID:    cm.ServerID,
							Measurement: "kiosk_display",
							Field:       fieldName,
							ValueInt:    &val,
							TagsJSON:    mustJSON(m.Tags),
						})
						continue
					}
					if fv, ok := toFloat64(raw); ok {
						val := fv
						points = append(points, models.SeriesPoint{
							Time:        ptTime,
							ServerID:    cm.ServerID,
							Measurement: "kiosk_display",
							Field:       fieldName,
							ValueDouble: &val,
							TagsJSON:    mustJSON(m.Tags),
						})
					}
				}
			}

		case "kiosk_power":
			powerType := strings.ToLower(m.Tags["type"])
			if powerType == "battery" {
				if batteryCaptured {
					continue
				}
				if presentVal, ok := toInt64(m.Fields["present"]); ok {
					cm.BatteryPresent = presentVal != 0
				}
				if chargeVal, ok := toInt64(m.Fields["charge_percent"]); ok {
					cm.BatteryChargePct = chargeVal
				}
				if voltageVal, ok := toInt64(m.Fields["voltage_mv"]); ok {
					cm.BatteryVoltageMV = voltageVal
				}
				if currentVal, ok := toInt64(m.Fields["current_ma"]); ok {
					cm.BatteryCurrentMA = currentVal
				}
				batteryCaptured = true
			} else {
				if powerOnlineCaptured {
					continue
				}
				if onlineVal, ok := toInt64(m.Fields["online"]); ok {
					cm.PowerOnline = onlineVal != 0
					powerOnlineCaptured = true
				}
			}

			if len(m.Fields) > 0 {
				for fieldName, raw := range m.Fields {
					if iv, ok := toInt64(raw); ok {
						val := iv
						points = append(points, models.SeriesPoint{
							Time:        ptTime,
							ServerID:    cm.ServerID,
							Measurement: "kiosk_power",
							Field:       fieldName,
							ValueInt:    &val,
							TagsJSON:    mustJSON(m.Tags),
						})
						continue
					}
					if fv, ok := toFloat64(raw); ok {
						val := fv
						points = append(points, models.SeriesPoint{
							Time:        ptTime,
							ServerID:    cm.ServerID,
							Measurement: "kiosk_power",
							Field:       fieldName,
							ValueDouble: &val,
							TagsJSON:    mustJSON(m.Tags),
						})
					}
				}
			}

		case "kiosk_input":
			device := models.InputDevice{
				Source: m.Tags["source"],
				Name:   m.Tags["name"],
				Vendor: m.Tags["vendor"],
				Product: m.Tags["product"],
				Bus:    m.Tags["bus"],
				Device: m.Tags["device"],
				Target: m.Tags["target"],
			}
			device.Identifier = firstNonEmpty(m.Tags["id"], m.Tags["identifier"], device.Name, device.Target, device.Device)

			present := false
			if presentVal, ok := toInt64(m.Fields["present"]); ok {
				present = presentVal != 0
			} else if eventVal, ok := toInt64(m.Fields["event_present"]); ok {
				present = eventVal != 0
			} else if linkVal, ok := toInt64(m.Fields["link_present"]); ok {
				present = linkVal != 0
			}
			device.Present = present
			if present {
				inputDevicesHealthy++
			} else {
				inputDevicesMissing++
			}
			inputDevices = append(inputDevices, device)

		case "kiosk_hotspot":
			if hotspotCaptured {
				continue
			}
			if tempRaw, ok := m.Fields["temp_c"]; ok {
				if tempVal, ok := toFloat64(tempRaw); ok {
					cm.HotspotTemperature = tempVal
					hotspotCaptured = true
					points = append(points,
						models.SeriesPoint{Time: ptTime, ServerID: cm.ServerID, Measurement: "kiosk_hotspot", Field: "temp_c", ValueDouble: &tempVal, TagsJSON: mustJSON(m.Tags)},
					)
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

		case "net":
			sawNetMetric = true
			iface := m.Tags["interface"]
			if iface == "" || strings.HasPrefix(iface, "lo") {
				continue
			}

			if v, ok := m.Fields["bytes_sent"].(float64); ok {
				value := int64(v)
				netBytesSent += value
				points = append(points,
					seriesPointIntValue(ptTime, cm.ServerID, "net", "bytes_sent", value, m.Tags),
				)
			}

			if v, ok := m.Fields["bytes_recv"].(float64); ok {
				value := int64(v)
				netBytesRecv += value
				points = append(points,
					seriesPointIntValue(ptTime, cm.ServerID, "net", "bytes_recv", value, m.Tags),
				)
			}

		case "temperature", "sensors", "kiosk_temperature":
			sawTemperatureMetric = true
			if tempValue, ok := extractTemperature(m.Fields); ok {
				if !temperatureCaptured {
					cm.Temperature = tempValue
					temperatureCaptured = true
				}
				points = append(points,
					seriesPointFloatValue(ptTime, cm.ServerID, "environment", "temperature_c", tempValue, m.Tags),
				)
			}

		case "kiosk_chassis":
			if chassisTemperatureCaptured {
				continue
			}
			if tempRaw, ok := m.Fields["temp_c"]; ok {
				if tempVal, ok := toFloat64(tempRaw); ok {
					cm.ChassisTemperature = tempVal
					chassisTemperatureCaptured = true
					points = append(points,
						models.SeriesPoint{Time: ptTime, ServerID: cm.ServerID, Measurement: "kiosk_chassis", Field: "temp_c", ValueDouble: &tempVal, TagsJSON: mustJSON(m.Tags)},
					)
				}
			}

		case "kiosk_fan":
			if fanCaptured {
				continue
			}
			if rpmRaw, ok := m.Fields["rpm"]; ok {
				if rpmVal, ok := toInt64(rpmRaw); ok {
					cm.FanRPM = rpmVal
					fanCaptured = true
					points = append(points,
						models.SeriesPoint{Time: ptTime, ServerID: cm.ServerID, Measurement: "kiosk_fan", Field: "rpm", ValueInt: &rpmVal, TagsJSON: mustJSON(m.Tags)},
					)
				}
			}

		case "vnstat_daily":
			if dailyVnstatCaptured {
				continue
			}
			rxBytes, rxOK := mibFieldToBytes(m.Fields, "rx_mib")
			txBytes, txOK := mibFieldToBytes(m.Fields, "tx_mib")
			if rxOK {
				cm.NetDailyRxBytes = rxBytes
			}
			if txOK {
				cm.NetDailyTxBytes = txBytes
			}
			if rxOK || txOK {
				dailyVnstatCaptured = true
				points = append(points,
					seriesPointIntValue(ptTime, cm.ServerID, "vnstat_daily", "rx_bytes", cm.NetDailyRxBytes, m.Tags),
					seriesPointIntValue(ptTime, cm.ServerID, "vnstat_daily", "tx_bytes", cm.NetDailyTxBytes, m.Tags),
				)
			}

		case "vnstat_monthly":
			if monthlyVnstatCaptured {
				continue
			}
			rxBytes, rxOK := mibFieldToBytes(m.Fields, "rx_mib")
			txBytes, txOK := mibFieldToBytes(m.Fields, "tx_mib")
			if rxOK {
				cm.NetMonthlyRxBytes = rxBytes
			}
			if txOK {
				cm.NetMonthlyTxBytes = txBytes
			}
			if rxOK || txOK {
				monthlyVnstatCaptured = true
				points = append(points,
					seriesPointIntValue(ptTime, cm.ServerID, "vnstat_monthly", "rx_bytes", cm.NetMonthlyRxBytes, m.Tags),
					seriesPointIntValue(ptTime, cm.ServerID, "vnstat_monthly", "tx_bytes", cm.NetMonthlyTxBytes, m.Tags),
				)
			}

		case "kiosk_volume":
			if volumeCaptured {
				continue
			}
			if v, ok := m.Fields["level_percent"]; ok {
				switch vv := v.(type) {
				case float64:
					cm.SoundVolumePercent = int64(vv)
					volumeCaptured = true
				case int64:
					cm.SoundVolumePercent = vv
					volumeCaptured = true
				case json.Number:
					if vi, err := vv.Int64(); err == nil {
						cm.SoundVolumePercent = vi
						volumeCaptured = true
					}
				}
				if volumeCaptured {
					val := cm.SoundVolumePercent
					if mutedVal, ok := m.Fields["muted"].(float64); ok {
						cm.SoundMuted = mutedVal != 0
					} else if mutedNum, ok := m.Fields["muted"].(json.Number); ok {
						if vi, err := mutedNum.Int64(); err == nil {
							cm.SoundMuted = vi != 0
						}
					}
					points = append(points,
						models.SeriesPoint{Time: ptTime, ServerID: cm.ServerID, Measurement: "kiosk_volume", Field: "level_percent", ValueInt: &val, TagsJSON: mustJSON(m.Tags)},
					)
					mutedInt := int64(0)
					if cm.SoundMuted {
						mutedInt = 1
					}
					points = append(points,
						models.SeriesPoint{Time: ptTime, ServerID: cm.ServerID, Measurement: "kiosk_volume", Field: "muted", ValueInt: &mutedInt, TagsJSON: mustJSON(m.Tags)},
					)
				}
			}
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

	cm.NetBytesSent = netBytesSent
	cm.NetBytesRecv = netBytesRecv
	if netBytesSent > 0 || netBytesRecv > 0 {
		ns := cm.NetBytesSent
		nr := cm.NetBytesRecv
		points = append(points,
			models.SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "net", Field: "bytes_sent_total", ValueInt: &ns, TagsJSON: []byte(`{"aggregated":true}`)},
			models.SeriesPoint{Time: cm.Time, ServerID: cm.ServerID, Measurement: "net", Field: "bytes_recv_total", ValueInt: &nr, TagsJSON: []byte(`{"aggregated":true}`)},
		)
	}

	cm.InputDevicesHealthy = inputDevicesHealthy
	cm.InputDevicesMissing = inputDevicesMissing
	if len(inputDevices) > 0 {
		cm.InputDevices = inputDevices
	} else {
		cm.InputDevices = nil
	}

	debugForServer := h.shouldLogForServer(cm.ServerID, payloadHost)

	if h.debugLoggingOn && debugForServer {
		log.Printf("ingest: parsed server_id=%s time=%s cpu=%.4f memory=%.4f temperature=%.2f chassis_temp=%.2f hotspot_temp=%.2f fan_rpm=%d volume_percent=%d muted=%t memory_total_bytes=%d memory_used_bytes=%d disk=%.4f disk_total_bytes=%d disk_used_bytes=%d disk_free_bytes=%d net_bytes_sent=%d net_bytes_recv=%d",
			cm.ServerID, cm.Time.UTC().Format(time.RFC3339), cm.CPU, cm.Memory,
			cm.Temperature, cm.ChassisTemperature, cm.HotspotTemperature, cm.FanRPM, cm.SoundVolumePercent, cm.SoundMuted, cm.MemoryTotalBytes, cm.MemoryUsedBytes, cm.Disk, cm.DiskTotalBytes, cm.DiskUsedBytes, cm.DiskFreeBytes, cm.NetBytesSent, cm.NetBytesRecv)

		if !sawTemperatureMetric {
			log.Printf("ingest: temperature measurement missing for server_id=%s", cm.ServerID)
		}
		if !temperatureCaptured && sawTemperatureMetric {
			log.Printf("ingest: temperature measurement present but unusable fields for server_id=%s fields=%v", cm.ServerID, payload.Metrics)
		}
		if !sawNetMetric {
			log.Printf("ingest: net measurement missing for server_id=%s", cm.ServerID)
		} else if netBytesSent == 0 && netBytesRecv == 0 {
			log.Printf("ingest: net measurement reported zero bytes for server_id=%s (check interfaces)", cm.ServerID)
		}
	}

	if h.debugLoggingOn && debugForServer {
		log.Printf("ingest: saving summary metric server_id=%s time=%s", cm.ServerID, cm.Time.UTC().Format(time.RFC3339))
	}
	if err := h.repo.SaveMetric(r.Context(), cm); err != nil {
		if h.debugLoggingOn && debugForServer {
			log.Printf("ingest: failed to save summary metric server_id=%s time=%s err=%v", cm.ServerID, cm.Time.UTC().Format(time.RFC3339), err)
		}
		WriteJSONError(w, http.StatusInternalServerError, "failed to persist metric: "+err.Error())
		return
	}
	if h.debugLoggingOn && debugForServer {
		log.Printf("ingest: saved summary metric server_id=%s time=%s", cm.ServerID, cm.Time.UTC().Format(time.RFC3339))
	}

	if len(points) > 0 {
		if h.directInsert || h.metricPoints == nil {
			if h.debugLoggingOn && debugForServer {
				log.Printf("ingest: writing %d series points for server_id=%s", len(points), cm.ServerID)
			}
			if err := h.repo.SaveSeriesPoints(r.Context(), points); err != nil {
				if h.debugLoggingOn && debugForServer {
					log.Printf("ingest: failed to save series points server_id=%s err=%v", cm.ServerID, err)
				}
				WriteJSONError(w, http.StatusInternalServerError, "failed to persist series points: "+err.Error())
				return
			}
			if h.debugLoggingOn && debugForServer {
				log.Printf("ingest: saved series points for server_id=%s", cm.ServerID)
			}
		} else {
			for _, p := range points {
				pointLog := h.shouldLogForServer(p.ServerID, payloadHost)
				if h.debugLoggingOn && pointLog {
					log.Printf("ingest: queueing point measurement=%s field=%s", p.Measurement, p.Field)
				}
				select {
				case h.metricPoints <- p:
					if h.debugLoggingOn && pointLog {
						log.Printf("ingest: queued point measurement=%s field=%s", p.Measurement, p.Field)
					}

				default:
					if h.debugLoggingOn && pointLog {
						log.Printf("ingest: failed to queue point measurement=%s field=%s", p.Measurement, p.Field)
					}
				}
			}
		}
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MetricsHandler) SeriesList(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")
	if serverID == "" {
		WriteJSONError(w, http.StatusBadRequest, "server_id required")
		return
	}

	p, err := parsePaginationParams(r, defaultPageSize, maxPageSize)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid pagination parameters")
		return
	}

	items, hasMore, err := h.repo.ListSeriesMeta(r.Context(), serverID, p.limit, p.offset)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, items, p.page, p.pageSize, hasMore)
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

	p, err := parsePaginationParams(r, defaultPageSize, maxPageSize)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid pagination parameters")
		return
	}

	out, hasMore, err := h.repo.SeriesQuery(r.Context(), serverID, measurement, field, rng, tagFilter, p.limit, p.offset)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, out, p.page, p.pageSize, hasMore)
}

func (h *MetricsHandler) Servers(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	region := r.URL.Query().Get("region")

	p, err := parsePaginationParams(r, defaultPageSize, maxPageSize)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid pagination parameters")
		return
	}

	servers, hasMore, err := h.repo.Servers(r.Context(), city, region, p.limit, p.offset)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, servers, p.page, p.pageSize, hasMore)
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

	p, err := parsePaginationParams(r, defaultPageSize, maxPageSize)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid pagination parameters")
		return
	}

	statuses, hasMore, err := h.repo.ServerStatus(r.Context(), city, region, p.limit, p.offset)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range statuses {
		age := time.Duration(statuses[i].AgeSeconds) * time.Second
		statuses[i].Online = age <= threshold
	}

	writePaginatedResponse(w, http.StatusOK, statuses, p.page, p.pageSize, hasMore)
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

	p, err := parsePaginationParams(r, defaultPageSize, maxPageSize)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid pagination parameters")
		return
	}

	items, hasMore, err := h.repo.CityStatusSummary(r.Context(), region, thresholdStr, p.limit, p.offset)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, items, p.page, p.pageSize, hasMore)
}

func (h *MetricsHandler) Latest(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, defaultPageSize, maxPageSize)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid pagination parameters")
		return
	}

	result, hasMore, err := h.repo.LatestMetrics(r.Context(), p.limit, p.offset)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, result, p.page, p.pageSize, hasMore)
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

	p, err := parsePaginationParams(r, defaultPageSize, maxPageSize)
	if err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid pagination parameters")
		return
	}

	result, hasMore, err := h.repo.HistoryMetrics(r.Context(), serverID, rng, p.limit, p.offset)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, result, p.page, p.pageSize, hasMore)
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

func seriesPointIntValue(t time.Time, serverID, measurement, field string, value int64, tags map[string]string) models.SeriesPoint {
	vv := value
	jb, _ := json.Marshal(tags)
	return models.SeriesPoint{Time: t, ServerID: serverID, Measurement: measurement, Field: field, ValueInt: &vv, TagsJSON: jb}
}

func seriesPointFloatValue(t time.Time, serverID, measurement, field string, value float64, tags map[string]string) models.SeriesPoint {
	vv := value
	jb, _ := json.Marshal(tags)
	return models.SeriesPoint{Time: t, ServerID: serverID, Measurement: measurement, Field: field, ValueDouble: &vv, TagsJSON: jb}
}

func mustJSON(tags map[string]string) []byte {
	if tags == nil {
		return []byte(`{}`)
	}
	if jb, err := json.Marshal(tags); err == nil {
		return jb
	}
	return []byte(`{}`)
}

func extractTemperature(fields map[string]interface{}) (float64, bool) {
	if fields == nil {
		return 0, false
	}

	getFloat := func(val interface{}) (float64, bool) {
		switch v := val.(type) {
		case float64:
			return v, true
		case int64:
			return float64(v), true
		case int:
			return float64(v), true
		case string:
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				return parsed, true
			}
		}
		return 0, false
	}

	preferredKeys := []string{"temp_input", "temperature", "temp_c", "temp", "value", "current"}
	for _, key := range preferredKeys {
		if val, ok := fields[key]; ok {
			if f, ok := getFloat(val); ok {
				return f, true
			}
		}
	}

	for key, val := range fields {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "temp") {
			if f, ok := getFloat(val); ok {
				return f, true
			}
		}
	}

	return 0, false
}

func toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func toInt64(val interface{}) (int64, bool) {
	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, true
		}
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}

func mibFieldToBytes(fields map[string]interface{}, key string) (int64, bool) {
	if fields == nil {
		return 0, false
	}
	val, ok := fields[key]
	if !ok {
		return 0, false
	}
	var f float64
	switch v := val.(type) {
	case float64:
		f = v
	case float32:
		f = float64(v)
	case int:
		f = float64(v)
	case int64:
		f = float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			f = parsed
		} else {
			return 0, false
		}
	default:
		return 0, false
	}
	bytes := int64(f * 1024 * 1024)
	if bytes < 0 {
		bytes = 0
	}
	return bytes, true
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

func (h *MetricsHandler) shouldLogForServer(serverID, hostTag string) bool {
	if !h.debugLoggingOn {
		return false
	}
	if h.debugServerID == "" {
		return true
	}
	target := strings.ToLower(h.debugServerID)
	if serverID != "" && strings.ToLower(serverID) == target {
		return true
	}
	if hostTag != "" && strings.ToLower(hostTag) == target {
		return true
	}
	return false
}
