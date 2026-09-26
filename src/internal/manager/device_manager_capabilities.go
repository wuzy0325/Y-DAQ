package manager

import (
	"yx-daq/internal/driver"
	"yx-daq/internal/types"
)

// UnitSetter 单位设置接口（仅XY-DAQ驱动实现）
type UnitSetter interface {
	SetUnit(unit string) error
}

// ThermocoupleTypeSetter 热电偶类型设置接口（仅 EA2516T 驱动实现）
type ThermocoupleTypeSetter interface {
	SetThermocoupleType(tcTypes string) error
	SetSingleThermocoupleType(channelIndex int, tcType string) error
}

// ValveController 校准阀控制接口（仅 EA2516A 压力驱动 + 模拟设备实现）
type ValveController interface {
	ReadValveState() (types.ValveState, error)
	SetValveState(state types.ValveState) error
}

// TempChannelConfigurator EA2508A 温度通道配置接口（@16 来源 + @17 热电偶类型，仅 EA2508A 驱动实现）
type TempChannelConfigurator interface {
	SetTempSource(source string) error
	SetTempThermocoupleType(tcType string) error
}

// AtmChannelConfigurator 大气压/温度采集使能配置接口（仅 EA2508A/EA2516A 驱动实现）
type AtmChannelConfigurator interface {
	SetAtmEnabled(enabled bool)
}

// ConfigSyncNotifier 配置同步通知能力接口（仅 EA2516T 驱动实现）
type ConfigSyncNotifier interface {
	OnConfigSynced(cb func(driver.DAQTHardwareConfig))
}

// StatusChangeNotifier 状态变更通知能力接口（仅 TCP 驱动实现）
type StatusChangeNotifier interface {
	SetOnStatusChange(func())
}
