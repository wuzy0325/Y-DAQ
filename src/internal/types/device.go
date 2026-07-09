package types

// DeviceType 设备类型标识
type DeviceType string

const (
	DeviceTypeSimulated DeviceType = "SIMULATED"
	DeviceTypeEA2508A   DeviceType = "EA2508A"
	DeviceTypeEA2516A   DeviceType = "EA2516A"
	DeviceTypeEA2516T   DeviceType = "EA2516T"
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
	DeviceTypeEA2508A: {
		Type: "EA2508A", Label: "EA2508A",
		PressureChCount: 8, TotalChCount: 10, FrameSize: 45,
		IsRealDAQ: true, DefaultHost: "192.168.3.101", DefaultPort: 9000, DefaultUnit: "kPa",
	},
	DeviceTypeEA2516A: {
		Type: "EA2516A", Label: "EA2516A",
		PressureChCount: 16, TotalChCount: 18, FrameSize: 77,
		IsRealDAQ: true, DefaultHost: "192.168.3.101", DefaultPort: 9000, DefaultUnit: "kPa",
	},
	DeviceTypeEA2516T: {
		Type: "EA2516T", Label: "EA2516T",
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
	return deviceTypeRegistry[DeviceTypeEA2516A] // 默认
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

// legacyDeviceTypeAliases 旧设备型号到新型号的迁移映射。
// 历史配置文件（~/.yx-daq/devices.json）中可能保存旧型号字符串（XY-DAQ8 / XY-DAQ16 / YX-DAQ-T），
// 升级后这些字符串无法命中 deviceTypeRegistry，会导致设备 fallback 到 EA2516A
// （8 通道硬件被当成 16 通道）且 driverFactories 查不到对应驱动无法连接。
// 在 DeviceManager.Init() 加载配置后调用 MigrateDeviceType 做一次性迁移并写回。
var legacyDeviceTypeAliases = map[DeviceType]DeviceType{
	"XY-DAQ8":  DeviceTypeEA2508A,
	"XY-DAQ16": DeviceTypeEA2516A,
	"YX-DAQ-T": DeviceTypeEA2516T,
}

// MigrateDeviceType 将旧设备型号字符串迁移到新型号。
// 返回 (mapped, changed)：changed=true 表示发生迁移，调用方需持久化。
// 已是新型号或不在别名表中的（如 SIMULATED）原样返回，changed=false。
func MigrateDeviceType(t DeviceType) (mapped DeviceType, changed bool) {
	if newType, ok := legacyDeviceTypeAliases[t]; ok {
		return newType, true
	}
	return t, false
}

// ConnectionStatus 连接状态
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "Disconnected"
	StatusConnecting   ConnectionStatus = "Connecting"
	StatusConnected    ConnectionStatus = "Connected"
	StatusError        ConnectionStatus = "Error"
)

// ValveState 校准阀状态（仅 EA2508A/EA2516A 压力设备支持）
// 阀位语义：校准位 = 传感器与校准口接通（数据无效），测量位 = 传感器与测量口接通（数据有效）
// 设备读阀返回 0 在不同固件下含义不一，未初始化归为 Unknown，由 UI 决定如何呈现
type ValveState string

const (
	ValveStateCalibration ValveState = "Calibration" // 校准位（设备读阀=1）
	ValveStateMeasurement ValveState = "Measurement" // 测量位（设备读阀=2/3）
	ValveStateUnknown     ValveState = "Unknown"     // 未初始化或读阀失败（设备读阀=0 或读阀异常）
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
	ThermocoupleType string  `json:"thermocoupleType,omitempty"` // 热电偶类型（K/J/T/E/N/S/R/B/C/WRE325/WRE526/WRE520），仅 EA2516T
	ZeroOffset       float64 `json:"zeroOffset,omitempty"`       // 零位偏移（校准时记录的当前读数，后续采集时减去）
	ZeroOffsetUnit   string  `json:"zeroOffsetUnit,omitempty"`   // 零位偏移记录时的单位（用于换单位后换算）
	ZeroCalibratedAt int64   `json:"zeroCalibratedAt,omitempty"` // 零位校准时刻（Unix 毫秒），0 表示未校准
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

// ZeroCalibrateResult 批量校零的单设备结果
type ZeroCalibrateResult struct {
	DeviceID   string `json:"deviceId"`   // 设备ID
	DeviceName string `json:"deviceName"` // 设备名称
	Success    bool   `json:"success"`    // 是否校零成功
	Channels   int    `json:"channels"`   // 校零的通道数（失败时为0）
	Error      string `json:"error"`      // 失败原因（成功时为空）
}
