package driver

const (
	streamMaskWithAtm      = "0810" // c 05 内容位图：压力 + 大气压 + 大气温度
	streamMaskPressureOnly = "0010" // c 05 内容位图：仅压力
)

// SetAtmEnabled 更新大气压/大气温度采集使能。
// 帧规格立即生效；c 05 命令在下次 StartAcquisition 时下发。
// 采集进行中禁止调用（由 DeviceManager.ensureConfigMutable 拦截）。
func (d *XYDAQDriver) SetAtmEnabled(enabled bool) {
	d.atmEnabled.Store(enabled)
}

func (d *XYDAQDriver) frameSpec() (channels, frameSize int) {
	if d.atmEnabled.Load() {
		return d.totalChannels, d.frameSize
	}
	return d.pressureCount, d.deviceType.PressureOnlyFrameSize()
}

func streamContentMask(atmEnabled bool) string {
	if atmEnabled {
		return streamMaskWithAtm
	}
	return streamMaskPressureOnly
}
