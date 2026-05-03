package types

// DeviceType 设备类型标识
type DeviceType string

const (
	DeviceTypeSimulated DeviceType = "SIMULATED"
	DeviceTypeXYDAQ8    DeviceType = "XY-DAQ8"
	DeviceTypeXYDAQ16   DeviceType = "XY-DAQ16"
)

// PressureChannelCount 返回该设备类型的压力通道数
func (t DeviceType) PressureChannelCount() int {
	switch t {
	case DeviceTypeXYDAQ8:
		return 8
	case DeviceTypeXYDAQ16:
		return 16
	default:
		return 16
	}
}

// TotalChannelCount 返回该设备类型的总通道数（压力+大气压+大气温度）
func (t DeviceType) TotalChannelCount() int {
	return t.PressureChannelCount() + 2
}

// StreamFrameSize 返回该设备类型的数据帧大小（字节）
func (t DeviceType) StreamFrameSize() int {
	return StreamFrameHeaderSize + t.TotalChannelCount()*4
}

// IsDAQDevice 是否为真实DAQ设备（非模拟）
func (t DeviceType) IsDAQDevice() bool {
	return t == DeviceTypeXYDAQ8 || t == DeviceTypeXYDAQ16
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
	RangeMax  float64 `json:"rangeMax"`
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
