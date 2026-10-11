package driver

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// AD 平均次数命令（参考 docs/hardware/硬件设备指令说明/DAQ-P-1604命令使用说明.md）：
//   - 读取：q05（响应为十六进制，如 "000A" 表示 10 次）
//   - 写入：w05<2 位十六进制>（响应 "A" 表示成功，Nxx 表示拒绝）
//
// 注意：文档 3.6 示例写作 w100A，但同页 w 命令索引表明确标注索引 05=AD平均次数，
// 且与 q05 读取索引一致，故此处采用索引 05。写入后通过 q05 回读校验，
// 若与实机协议不符会以错误形式暴露（不会静默成功）。
const adAvgCmdIndex = "05"

// applyADAveraging 连接后下发 AD 平均次数。
// 先读取 q05 确认设备支持该命令，读取成功且当前值与目标不一致时才写入，
// 避免对不支持 AD 平均命令的设备误写。失败仅告警，不阻断连接。
func (d *XYDAQDriver) applyADAveraging() {
	desired := int(d.adAvgCount.Load())
	if desired <= 0 {
		return
	}
	cur, err := d.readRawADAveraging()
	if err != nil {
		slog.Warn("XY-DAQ read AD averaging failed, skip apply",
			"host", d.Host, "port", d.Port, "err", err)
		return
	}
	if cur == desired {
		return
	}
	if err := d.SetADAveraging(desired); err != nil {
		slog.Warn("XY-DAQ apply AD averaging failed",
			"host", d.Host, "port", d.Port, "want", desired, "err", err)
	}
}

// ReadADAveraging 读取设备当前 AD 平均次数（q05，十六进制响应）。
func (d *XYDAQDriver) ReadADAveraging() (int, error) {
	return d.readRawADAveraging()
}

// readRawADAveraging 读取 AD 平均次数，不修改驱动缓存。
func (d *XYDAQDriver) readRawADAveraging() (int, error) {
	resp, err := d.sendUnitCommand("q" + adAvgCmdIndex)
	if err != nil {
		return 0, fmt.Errorf("读取AD平均次数失败: %w", err)
	}
	return parseADAveragingResponse(resp)
}

// SetADAveraging 设置设备 AD 平均次数（w05<2 位十六进制>），写入后回读校验。
// 成功后更新驱动缓存；回读不一致返回错误（缓存同步为设备实际值）。
func (d *XYDAQDriver) SetADAveraging(count int) error {
	if count < 1 || count > 0xFF {
		return fmt.Errorf("AD平均次数超出范围: %d（合法 1-255）", count)
	}
	cmd := fmt.Sprintf("w%s%02X", adAvgCmdIndex, count)
	resp, err := d.sendUnitCommand(cmd)
	if err != nil {
		return fmt.Errorf("设置AD平均次数失败: %w", err)
	}
	trimmed := strings.TrimSpace(resp)
	if isNACK(trimmed) {
		return fmt.Errorf("设备拒绝设置AD平均次数: %s", trimmed)
	}
	if trimmed != "A" {
		return fmt.Errorf("设置AD平均次数响应异常: %q", trimmed)
	}

	// readback 校验：确认设备实际接受的值
	if got, readErr := d.readRawADAveraging(); readErr == nil {
		d.adAvgCount.Store(int32(got))
		if got != count {
			return fmt.Errorf("AD平均次数回读不一致: 已发送 %d, 设备返回 %d", count, got)
		}
		return nil
	}
	d.adAvgCount.Store(int32(count))
	return nil
}

// parseADAveragingResponse 解析 q05 响应为十进制次数。
// 抽为纯函数便于单测覆盖（NACK / 带 A 前缀 / 裸十六进制 / 非法）。
func parseADAveragingResponse(resp string) (int, error) {
	raw := strings.TrimSpace(resp)
	if isNACK(raw) {
		return 0, fmt.Errorf("设备拒绝读取AD平均次数: %s", raw)
	}
	val := strings.TrimSpace(strings.TrimPrefix(raw, "A")) // 部分固件带 A 前缀
	if val == "" {
		return 0, fmt.Errorf("AD平均次数响应为空: %q", resp)
	}
	n, err := strconv.ParseInt(val, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("AD平均次数响应非法: %q", resp)
	}
	return int(n), nil
}
