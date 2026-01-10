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

type InputDevice struct {
	Source     string `json:"source"`
	Identifier string `json:"identifier"`
	Name       string `json:"name,omitempty"`
	Vendor     string `json:"vendor,omitempty"`
	Product    string `json:"product,omitempty"`
	Bus        string `json:"bus,omitempty"`
	Device     string `json:"device,omitempty"`
	Target     string `json:"target,omitempty"`
	Present    bool   `json:"present"`
}

type LinkState struct {
	Interface       string `json:"interface,omitempty"`
	Type            string `json:"type,omitempty"`
	LinkUp          bool   `json:"link_up"`
	SpeedMbps       int64  `json:"speed_mbps,omitempty"`
	DuplexFull      bool   `json:"duplex_full"`
	Autoneg         bool   `json:"autoneg"`
	RxErrors        int64  `json:"rx_errors,omitempty"`
	TxErrors        int64  `json:"tx_errors,omitempty"`
	RxDropped       int64  `json:"rx_dropped,omitempty"`
	TxDropped       int64  `json:"tx_dropped,omitempty"`
	SignalDbm       int64  `json:"signal_dbm,omitempty"`
	TxBitrateMbps   int64  `json:"tx_bitrate_mbps,omitempty"`
	RxBitrateMbps   int64  `json:"rx_bitrate_mbps,omitempty"`
}

type ProcessStatus struct {
	Name         string `json:"name"`
	Running      bool   `json:"running"`
	ProcessCount int64  `json:"process_count,omitempty"`
}

type CleanMetric struct {
	ServerID           string
	CPU                float64
	Memory             float64
	Temperature        float64
	ChassisTemperature float64
	HotspotTemperature float64
	PowerOnline        bool
	BatteryPresent     bool
	BatteryChargePct   int64
	BatteryVoltageMV   int64
	BatteryCurrentMA   int64
	SoundVolumePercent int64
	SoundMuted         bool
	DisplayConnected   bool
	DisplayWidth       int64
	DisplayHeight      int64
	DisplayRefreshHz   int64
	DisplayPrimary     bool
	DisplayDpmsEnabled bool
	FanRPM             int64
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
	InputDevicesHealthy int64
	InputDevicesMissing int64
	InputDevices        []InputDevice
	LinkState          *LinkState
	ProcessStatuses    []ProcessStatus
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
	ChassisTemperature float64   `json:"chassis_temp_c"`
	HotspotTemperature float64   `json:"hotspot_temp_c"`
	PowerOnline        bool      `json:"power_online"`
	BatteryPresent     bool      `json:"battery_present"`
	BatteryChargePct   int64     `json:"battery_charge_percent"`
	BatteryVoltageMV   int64     `json:"battery_voltage_mv"`
	BatteryCurrentMA   int64     `json:"battery_current_ma"`
	SoundVolumePercent int64     `json:"sound_volume_percent"`
	SoundMuted         bool      `json:"sound_muted"`
	DisplayConnected   bool      `json:"display_connected"`
	DisplayWidth       int64     `json:"display_width"`
	DisplayHeight      int64     `json:"display_height"`
	DisplayRefreshHz   int64     `json:"display_refresh_hz"`
	DisplayPrimary     bool      `json:"display_primary"`
	DisplayDpmsEnabled bool      `json:"display_dpms_enabled"`
	FanRPM             int64     `json:"fan_rpm"`
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
	InputDevicesHealthy int64        `json:"input_devices_healthy"`
	InputDevicesMissing int64        `json:"input_devices_missing"`
	InputDevices        []InputDevice `json:"input_devices"`
	LinkState           *LinkState   `json:"link_state,omitempty"`
	ProcessStatuses     []ProcessStatus `json:"process_statuses,omitempty"`
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
	ChassisTemperature float64   `json:"chassis_temp_c"`
	HotspotTemperature float64   `json:"hotspot_temp_c"`
	PowerOnline        bool      `json:"power_online"`
	BatteryPresent     bool      `json:"battery_present"`
	BatteryChargePct   int64     `json:"battery_charge_percent"`
	BatteryVoltageMV   int64     `json:"battery_voltage_mv"`
	BatteryCurrentMA   int64     `json:"battery_current_ma"`
	SoundVolumePercent int64     `json:"sound_volume_percent"`
	SoundMuted         bool      `json:"sound_muted"`
	DisplayConnected   bool      `json:"display_connected"`
	DisplayWidth       int64     `json:"display_width"`
	DisplayHeight      int64     `json:"display_height"`
	DisplayRefreshHz   int64     `json:"display_refresh_hz"`
	DisplayPrimary     bool      `json:"display_primary"`
	DisplayDpmsEnabled bool      `json:"display_dpms_enabled"`
	FanRPM             int64     `json:"fan_rpm"`
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
	NetMonthlyRxBytes  int64        `json:"net_monthly_rx_bytes"`
	NetMonthlyTxBytes  int64        `json:"net_monthly_tx_bytes"`
	InputDevicesHealthy int64        `json:"input_devices_healthy"`
	InputDevicesMissing int64        `json:"input_devices_missing"`
	InputDevices        []InputDevice `json:"input_devices"`
	LinkState           *LinkState   `json:"link_state,omitempty"`
	ProcessStatuses     []ProcessStatus `json:"process_statuses,omitempty"`
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
