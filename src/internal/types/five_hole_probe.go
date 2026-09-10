package types

import "fmt"

// ==================== 五孔探针通道角色 ====================

// FiveHoleChannelRole 五孔探针通道语义角色
type FiveHoleChannelRole string

const (
	Role5H_P1   FiveHoleChannelRole = "fiveHole.p1"   // 1号孔压力
	Role5H_P2   FiveHoleChannelRole = "fiveHole.p2"   // 2号孔压力（中心孔）
	Role5H_P3   FiveHoleChannelRole = "fiveHole.p3"   // 3号孔压力
	Role5H_P4   FiveHoleChannelRole = "fiveHole.p4"   // 4号孔压力
	Role5H_P5   FiveHoleChannelRole = "fiveHole.p5"   // 5号孔压力
	Role5H_PAtm FiveHoleChannelRole = "fiveHole.pAtm" // 大气压
	Role5H_TAtm FiveHoleChannelRole = "fiveHole.tAtm" // 气流温度
)

// FiveHoleProbeChannelConfig 五孔探针通道配置（每通道独立选采集设备）
type FiveHoleProbeChannelConfig struct {
	Name     string              `json:"name"`
	Role     FiveHoleChannelRole `json:"role"`
	DeviceID string              `json:"deviceId"` // 每通道独立选采集设备
	Channel  int                 `json:"channel"`
	Enabled  bool                `json:"enabled"`
}

// ==================== 五孔运动轴映射 ====================

// FiveHoleMotionAxisMapping 五孔运动轴映射（每轴独立选位移机构）
// 与三孔 MotionAxisMapping 区别：含 ControllerID，支持每轴独立位移机构
type FiveHoleMotionAxisMapping struct {
	ControllerID string  `json:"controllerId"` // 位移机构ID
	Axis        AxisName `json:"axis"`         // 轴名（X/Y/Z/U）
}

// ==================== 五孔校准文件 ====================

// FiveHoleCalibRange 五孔校准文件有效范围
type FiveHoleCalibRange struct {
	AlphaMin float64 `json:"alphaMin"` // 最小攻角（度）
	AlphaMax float64 `json:"alphaMax"` // 最大攻角（度）
	BetaMin  float64 `json:"betaMin"`  // 最小侧滑角（度）
	BetaMax  float64 `json:"betaMax"`  // 最大侧滑角（度）
	MachMin  float64 `json:"machMin"`  // 最小马赫数
	MachMax  float64 `json:"machMax"`  // 最大马赫数
}

// FiveHoleCalibFileInfo 五孔校准文件信息（.cal 文件：首行 13 13，后接 169 行 ka kb cpt cps alpha beta）
type FiveHoleCalibFileInfo struct {
	FilePath   string             `json:"filePath"`
	FileName   string             `json:"fileName"`
	CMa        float64            `json:"cMa"`        // 校准马赫数（从文件名解析）
	ValidRange FiveHoleCalibRange `json:"validRange"` // 有效范围（α/β/Ma）
}

// ==================== 五孔大气数据源（每探针独立） ====================

// FiveHoleAtmSourceMode 大气压/气流温度数据源模式
type FiveHoleAtmSourceMode string

const (
	// FiveHoleSourceDevice 设备读取（压力扫描阀上的大气压/大气温度通道，或温度扫描阀的通道）
	FiveHoleSourceDevice FiveHoleAtmSourceMode = "device"
	// FiveHoleSourceManual 手动写入固定值
	FiveHoleSourceManual FiveHoleAtmSourceMode = "manual"
)

// FiveHoleAtmSource 大气压/气流温度数据源（每探针独立配置）
type FiveHoleAtmSource struct {
	Mode        FiveHoleAtmSourceMode `json:"mode"`        // device=设备读取 / manual=手动写入
	DeviceID    string                `json:"deviceId"`    // Mode=device 时有效
	Channel     int                   `json:"channel"`     // Mode=device 时有效
	ManualValue float64               `json:"manualValue"` // Mode=manual 时有效
}

// validate 校验数据源配置（name 用于错误信息，如"大气压"/"气流温度"）
// Mode 为空视为 device（兼容旧配置缺省值）
func (s FiveHoleAtmSource) validate(name string) error {
	switch s.Mode {
	case FiveHoleSourceManual:
		return nil
	case FiveHoleSourceDevice, "":
		if s.DeviceID == "" {
			return fmt.Errorf("%s数据源未选择采集设备", name)
		}
		if s.Channel < 0 {
			return fmt.Errorf("%s数据源通道号必须≥0", name)
		}
		return nil
	default:
		return fmt.Errorf("%s数据源模式无效: %s", name, s.Mode)
	}
}

// ==================== 五孔探针配置 ====================

// FiveHoleProbeConfig 单根五孔探针配置
type FiveHoleProbeConfig struct {
	ProbeID       string                       `json:"probeId"`       // probe1..probeN (N≤MaxFiveHoleProbes，前端动态增删，复用最小未用序号)
	Enabled       bool                         `json:"enabled"`       // 是否启用（配几根跑几根）
	ProbeChannels []FiveHoleProbeChannelConfig `json:"probeChannels"` // P1-P5 各自数据源
	MotionX       FiveHoleMotionAxisMapping    `json:"motionX"`       // X 方向：位移机构 + 轴号
	MotionY       FiveHoleMotionAxisMapping    `json:"motionY"`       // Y 方向：位移机构 + 轴号
	CalibFiles    []FiveHoleCalibFileInfo       `json:"calibFiles"`    // .cal 校准文件（每探针独立载入）
	// 大气压/气流温度数据源（每探针独立，不再全局共享）
	PAtmSource    FiveHoleAtmSource            `json:"pAtmSource"`    // 大气压 P∞：设备读取或手动写入
	TAtmSource    FiveHoleAtmSource            `json:"tAtmSource"`    // 气流温度 T∞：设备读取或手动写入
}
