package driver

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// AD 平均次数命令（真机实测校准）：
// 读取用 q05，响应为 4 位十六进制（"0004"=4、"0010"=16）；写入用 "w10" + 2 位十六进制值
// （4→w1004、8→w1008、16→w1010、32→w1020），成功回 "A"，拒绝回 Nxx。
//
// 值必须恰好 2 位十六进制：曾误用 4 位（如 w101004），设备会把多出的高位一起当值、
// 实际按 w10+10（=16）解析——虽同样回 "A" 但设置不生效。因此写入后一律用 q05 回读
// 比对（ACK 为 "A" 只代表命令被接受，不代表值被采纳）：实测
// w101004/w101008/w101010/w101020 全部回 "A" 而 q05 仍是原值。
//
// 连接时不反向下发，改为读取设备实际值同步到配置/界面（见 DeviceManager.Connect）。
const (
	// adAvgReadCmd AD 平均次数读取命令（索引 05，响应 4 位十六进制）
	adAvgReadCmd = "q05"
	// adAvgWritePrefix AD 平均次数写入命令前缀（索引 10）
	adAvgWritePrefix = "w10"
)

// adAvgWriteCmd 构造 AD 平均写命令：w10 + 2 位十六进制次数。
// 如 4 → w1004、8 → w1008、16 → w1010、32 → w1020。
// 值域 1-255 恰好 2 位十六进制，与设备解析宽度一致（多余位会被设备误并进数值）。
func adAvgWriteCmd(count int) string {
	return fmt.Sprintf("%s%02X", adAvgWritePrefix, count)
}

// ReadADAveraging 读取设备当前 AD 平均次数（q05，十六进制响应）。
// 连接后由 DeviceManager 调用，以设备实际值同步配置与界面（不在连接时反向下发）。
func (d *XYDAQDriver) ReadADAveraging() (int, error) {
	return d.readRawADAveraging()
}

// readRawADAveraging 读取 AD 平均次数。
func (d *XYDAQDriver) readRawADAveraging() (int, error) {
	resp, err := d.sendUnitCommand(adAvgReadCmd)
	if err != nil {
		return 0, fmt.Errorf("读取AD平均次数失败: %w", err)
	}
	return parseADAveragingResponse(resp)
}

// SetADAveraging 设置设备 AD 平均次数（w10<4 位十六进制值>），写入后回读校验。
//
// 真机实测：写入成功回 "A"，拒绝回 Nxx；应答不符合预期即判失败（不静默通过）。
// 成功后再用 q05 回读确认设备实际接受的值（防止设备钳制取值）。
func (d *XYDAQDriver) SetADAveraging(count int) error {
	if count < 1 || count > 0xFF {
		return fmt.Errorf("AD平均次数超出范围: %d（合法 1-255）", count)
	}
	cmd := adAvgWriteCmd(count)
	resp, err := d.sendUnitCommand(cmd)
	if err != nil {
		return fmt.Errorf("设置AD平均次数失败: %w", err)
	}
	if err := interpretADAveragingWriteResponse(resp); err != nil {
		return err
	}

	// readback 校验：以设备实际值为准（设备可能接受命令但钳制取值）
	got, readErr := d.readRawADAveraging()
	if readErr != nil {
		// 回读不可用（个别固件不支持 q05）不推翻已确认的 "A" 应答
		slog.Warn("AD averaging readback failed after write, assume applied",
			"host", d.Host, "port", d.Port, "cmd", cmd, "want", count, "err", readErr)
		return nil
	}
	if got != count {
		return fmt.Errorf("AD平均次数回读不一致: 已发送 %d, 设备返回 %d", count, got)
	}
	return nil
}

// interpretADAveragingWriteResponse 解析写入应答：真机实测成功回 "A"，
// 拒绝回 Nxx（如 N07）。其余应答视为协议异常，明确报错而非静默通过。
func interpretADAveragingWriteResponse(resp string) error {
	trimmed := strings.TrimSpace(resp)
	if isNACK(trimmed) {
		return fmt.Errorf("设备拒绝设置AD平均次数: %s", trimmed)
	}
	if trimmed != "A" {
		return fmt.Errorf("设置AD平均次数响应异常: %q（期望 A）", trimmed)
	}
	return nil
}

// parseADAveragingResponse 解析 q05 响应为十进制次数。
//
// resp 是长度前缀帧剥离后的 ASCII 载荷（见 sendUnitCommand / processData），实测形如
// "0004"/"0010"。这里额外防御写命令 ACK 与本响应串台：粘连形态 "A0004" 剥离前缀后
// 取值；单独残留的裸 "A" 判为 ACK 残留报错——否则会被当成十六进制 10 次显示到界面。
func parseADAveragingResponse(resp string) (int, error) {
	raw := strings.TrimSpace(resp)
	if isNACK(raw) {
		return 0, fmt.Errorf("设备拒绝读取AD平均次数: %s", raw)
	}
	if raw == "" {
		return 0, fmt.Errorf("AD平均次数响应为空: %q", resp)
	}
	// 裸 "A" 只可能是写命令 ACK（q05 正常回 4 位零填充十六进制）
	if strings.EqualFold(raw, "A") {
		return 0, fmt.Errorf("AD平均次数响应为写命令ACK残留: %q", resp)
	}

	// 优先按完整响应解析十六进制：既覆盖零填充的 "000A"(=10)，也覆盖
	// 首位为 A 的合法值（如裸十六进制 "A0"=160，不能被当作前缀误剥离）。
	n, err := strconv.ParseInt(raw, 16, 32)
	if err == nil && n <= 0xFF {
		return int(n), nil
	}

	// 写命令 ACK "A" 与 q05 响应粘连（"A000A"=10）时剥离前缀重试。仅当整体解析
	// 失败、或整体解析值超出 AD 平均有效范围（>255，说明首位 A 是 ACK 而非数值
	// nibble）时才剥离。
	if strings.HasPrefix(raw, "A") {
		if stripped := strings.TrimSpace(raw[1:]); stripped != "" {
			if n2, err2 := strconv.ParseInt(stripped, 16, 32); err2 == nil {
				return int(n2), nil
			}
		}
	}

	if err != nil {
		return 0, fmt.Errorf("AD平均次数响应非法: %q", resp)
	}
	return 0, fmt.Errorf("AD平均次数响应超出范围: %q", resp)
}
