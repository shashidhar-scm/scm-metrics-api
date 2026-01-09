package models

import "time"

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
	ServerID           string
	CPU                float64
	Memory             float64
	Temperature        float64
	SoundVolumePercent int64
	SoundMuted         bool
	MemoryTotalBytes   int64
	MemoryUsedBytes    int64
	Disk               float64
	DiskTotalBytes     int64
	DiskUsedBytes      int64
	DiskFreeBytes      int64
	NetBytesSent       int64
	NetBytesRecv       int64
	NetDailyRxBytes    int64
	NetDailyTxBytes    int64
	NetMonthlyRxBytes  int64
	NetMonthlyTxBytes  int64
	Uptime             int64
	City               string
	CityName           string
	Region             string
	RegionName         string
	Time               time.Time
}

type LatestMetric struct {
	ServerID           string    `json:"server_id"`
	Time               time.Time `json:"time"`
	CPU                float64   `json:"cpu"`
	Memory             float64   `json:"memory"`
	Temperature        float64   `json:"temperature"`
	SoundVolumePercent int64     `json:"sound_volume_percent"`
	SoundMuted         bool      `json:"sound_muted"`
	MemoryTotalBytes   int64     `json:"memory_total_bytes"`
	MemoryUsedBytes    int64     `json:"memory_used_bytes"`
	Disk               float64   `json:"disk"`
	DiskTotalBytes     int64     `json:"disk_total_bytes"`
	DiskUsedBytes      int64     `json:"disk_used_bytes"`
	DiskFreeBytes      int64     `json:"disk_free_bytes"`
	NetBytesSent       int64     `json:"net_bytes_sent"`
	NetBytesRecv       int64     `json:"net_bytes_recv"`
	NetDailyRxBytes    int64     `json:"net_daily_rx_bytes"`
	NetDailyTxBytes    int64     `json:"net_daily_tx_bytes"`
	NetMonthlyRxBytes  int64     `json:"net_monthly_rx_bytes"`
	NetMonthlyTxBytes  int64     `json:"net_monthly_tx_bytes"`
	Uptime             int64     `json:"uptime"`
	City               string    `json:"city"`
	CityName           string    `json:"city_name"`
	Region             string    `json:"region"`
	RegionName         string    `json:"region_name"`
}

type HistoryMetric struct {
	Time               time.Time `json:"time"`
	CPU                float64   `json:"cpu"`
	Memory             float64   `json:"memory"`
	Temperature        float64   `json:"temperature"`
	SoundVolumePercent int64     `json:"sound_volume_percent"`
	SoundMuted         bool      `json:"sound_muted"`
	MemoryTotalBytes   int64     `json:"memory_total_bytes"`
	MemoryUsedBytes    int64     `json:"memory_used_bytes"`
	Disk               float64   `json:"disk"`
	DiskTotalBytes     int64     `json:"disk_total_bytes"`
	DiskUsedBytes      int64     `json:"disk_used_bytes"`
	DiskFreeBytes      int64     `json:"disk_free_bytes"`
	NetBytesSent       int64     `json:"net_bytes_sent"`
	NetBytesRecv       int64     `json:"net_bytes_recv"`
	NetDailyRxBytes    int64     `json:"net_daily_rx_bytes"`
	NetDailyTxBytes    int64     `json:"net_daily_tx_bytes"`
	NetMonthlyRxBytes  int64     `json.htmlon":"net_monthly_rx_bytes"`
	NetMonthlyTxBytes  int64     `json:"net_monthly_tx_bytes"`
	Uptime             int64     `json:"uptime"`
	City               string    `json:"city"`
	CityName           string    `json:"city_name"`
	Region             string    `json:"region"`
	RegionName         string    `json:"region_name"`
}

type SeriesMeta struct {
	Measurement string `json:"measurement"`
	Field       string `json:"field"`
}

type SeriesPointResponse struct {
	Time        time.Time              `json:"time"`
	ServerID    string                 `json:"server_id"`
	Measurement string                 `json:"measurement"`
	Field       string                 `json:"field"`
	ValueDouble *float64               `json:"value_double,omitempty"`
	ValueInt    *int64                 `json:"value_int,omitempty"`
	Tags        map[string]interface{} `json:"tags"`
}

type ServerStatus struct {
	ServerID   string    `json:"server_id"`
	LastSeen   time.Time `json:"last_seen"`
	AgeSeconds int64     `json:"age_seconds"`
	Online     bool      `json:"online"`
	City       string    `json:"city"`
	CityName   string    `json:"city_name"`
	Region     string    `json:"region"`
	RegionName string    `json:"region_name"`
}

type CityStatusSummary struct {
	City         string `json:"city"`
	CityName     string `json:"city_name"`
	OnlineCount  int64  `json:"online"`
	OfflineCount int64  `json:"offline"`
	Total        int64  `json:"total"`
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
