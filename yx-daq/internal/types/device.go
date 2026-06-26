package types

// DeviceType 设备类型标识
type DeviceType string

const (
	DeviceTypeSimulated DeviceType = "SIMULATED"
	DeviceTypeXYDAQ8    DeviceType = "XY-DAQ8"
	DeviceTypeXYDAQ16   DeviceType = "XY-DAQ16"
	DeviceTypeYXDAQT    DeviceType = "YX-DAQ-T"
)

// DeviceTypeInfo 设备类型元数据（注册表驱动，新增设备类型只需加一行）
type DeviceTypeInfo struct {
	Type            DeviceType
	Label           string
	PressureChCount int  // 压力/主通道数
	TotalChCount    int  // 总通道数
	FrameSize       int  // 数据帧大小（0 = 驱动自定义）
	IsTemperature   bool // 是否为温度采集设备
	IsRealDAQ       bool // 是否为真实 DAQ 设备（非模拟）
	DefaultHost     string
	DefaultPort     int
	DefaultUnit     string // 主通道默认单位
}

// deviceTypeRegistry 设备类型注册表 — 新增设备类型只需在此添加一行
var deviceTypeRegistry = map[DeviceType]DeviceTypeInfo{
	DeviceTypeXYDAQ8: {
		Type: "XY-DAQ8", Label: "XY-DAQ8",
		PressureChCount: 8, TotalChCount: 10, FrameSize: 45,
		IsRealDAQ: true, DefaultHost: "192.168.3.101", DefaultPort: 9000, DefaultUnit: "kPa",
	},
	DeviceTypeXYDAQ16: {
		Type: "XY-DAQ16", Label: "XY-DAQ16",
		PressureChCount: 16, TotalChCount: 18, FrameSize: 77,
		IsRealDAQ: true, DefaultHost: "192.168.3.101", DefaultPort: 9000, DefaultUnit: "kPa",
	},
	DeviceTypeYXDAQT: {
		Type: "YX-DAQ-T", Label: "DAQ-T-1603",
		PressureChCount: 16, TotalChCount: 16, FrameSize: 0,
		IsTemperature: true, IsRealDAQ: true,
		DefaultHost: "192.168.1.7", DefaultPort: 9000, DefaultUnit: "°C",
	},
	DeviceTypeSimulated: {
		Type: "SIMULATED", Label: "模拟设备",
		PressureChCount: 16, TotalChCount: 18, FrameSize: 77,
		DefaultUnit: "kPa",
	},
}

// Info 返回该设备类型的元数据（注册表查询，无需 switch）
func (t DeviceType) Info() DeviceTypeInfo {
	if info, ok := deviceTypeRegistry[t]; ok {
		return info
	}
	return deviceTypeRegistry[DeviceTypeXYDAQ16] // 默认
}

// 以下方法委托给 Info()，保持向后兼容

// PressureChannelCount 返回该设备类型的压力通道数
func (t DeviceType) PressureChannelCount() int {
	return t.Info().PressureChCount
}

// TotalChannelCount 返回该设备类型的总通道数
func (t DeviceType) TotalChannelCount() int {
	return t.Info().TotalChCount
}

// StreamFrameSize 返回该设备类型的数据帧大小（字节）
func (t DeviceType) StreamFrameSize() int {
	return t.Info().FrameSize
}

// IsDAQDevice 是否为真实DAQ设备（非模拟）
func (t DeviceType) IsDAQDevice() bool {
	return t.Info().IsRealDAQ
}

// IsTemperatureDevice 是否为温度采集设备（热电偶）
func (t DeviceType) IsTemperatureDevice() bool {
	return t.Info().IsTemperature
}

// AllDeviceTypes 返回所有已注册设备类型（供前端枚举使用）
func AllDeviceTypes() []DeviceTypeInfo {
	result := make([]DeviceTypeInfo, 0, len(deviceTypeRegistry))
	for _, info := range deviceTypeRegistry {
		result = append(result, info)
	}
	return result
}

// ConnectionStatus 连接状态
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "Disconnected"
	StatusConnecting   ConnectionStatus = "Connecting"
	StatusConnected    ConnectionStatus = "Connected"
	StatusError        ConnectionStatus = "Error"
)

// ChannelConfig 通道配置
type ChannelConfig struct {
	Index     int     `json:"index"`
	Name      string  `json:"name"`
	Enabled   bool    `json:"enabled"`
	Unit      string  `json:"unit"`
	Precision int     `json:"precision"`
	RangeMin  float64 `json:"rangeMin"`
	RangeMax         float64 `json:"rangeMax"`
	ThermocoupleType string  `json:"thermocoupleType,omitempty"` // 热电偶类型（K/J/T/E/N/S/R/B/C/WRE325/WRE526/WRE520），仅 YX-DAQ-T
}

// DeviceProfile 设备完整配置
type DeviceProfile struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        DeviceType      `json:"type"`
	Host        string          `json:"host"`
	Port        int             `json:"port"`
	StreamID    int             `json:"streamId"`
	PeriodMs    int             `json:"periodMs"`    // 采集周期(毫秒)，0表示使用默认50ms
	AutoConnect bool            `json:"autoConnect"` // 是否自动连接
	Channels    []ChannelConfig `json:"channels"`
}

// DeviceInstance 运行时设备实例
type DeviceInstance struct {
	ProfileID string           `json:"profileId"`
	Status    ConnectionStatus `json:"status"`
	Acquiring bool             `json:"acquiring"`
	LastError string           `json:"lastError"`
}

// DataPayload 数据帧
type DataPayload struct {
	DeviceID       string    `json:"deviceId"`
	Timestamp      int64     `json:"timestamp"`
	Channels       []float64 `json:"channels"`
	ChannelIndices []int     `json:"channelIndices"`
	ChannelUnits   []string  `json:"channelUnits"`
}

// DiscoveredDevice UDP扫描发现的设备
type DiscoveredDevice struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	SN       string `json:"sn"`
	Firmware string `json:"firmware"`
	Port     int    `json:"port"`
	Mask     string `json:"mask"`
	Gateway  string `json:"gateway"`
}

// DataCallback 数据回调函数类型
type DataCallback func(payload DataPayload)

// DeviceStatus 设备状态摘要
type DeviceStatus struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Type      DeviceType       `json:"type"`
	Status    ConnectionStatus `json:"status"`
	Acquiring bool             `json:"acquiring"`
	LastError string           `json:"lastError"`
}
