package driver

import (
	"testing"

	"yx-daq/internal/types"
)

// TestParseValveReadResponse_NACK 验证设备拒绝错误码（Nxx）作为错误抛出
// 不能误把 "N09" 当成阀位 0，否则校准位被错判为测量位导致无效数据被采入
func TestParseValveReadResponse_NACK(t *testing.T) {
	cases := []string{"N09", "N03", "N99", "N00"}
	for _, resp := range cases {
		t.Run(resp, func(t *testing.T) {
			got, err := parseValveReadResponse(resp)
			if err == nil {
				t.Fatalf("expected error for NACK %q, got state=%v err=nil", resp, got)
			}
			if got != types.ValveStateUnknown {
				t.Fatalf("NACK should map to Unknown, got %v", got)
			}
		})
	}
}

// TestParseValveReadResponse_Numeric 数字 1/2/3 映射，0 归为 Unknown
func TestParseValveReadResponse_Numeric(t *testing.T) {
	cases := []struct {
		resp  string
		want  types.ValveState
		isErr bool
	}{
		{"A1", types.ValveStateCalibration, false},
		{"A2", types.ValveStateMeasurement, false},
		{"A3", types.ValveStateMeasurement, false}, // 现场兼容：部分固件 RUN/测量态返回 3
		{"A0", types.ValveStateUnknown, false},    // 0 在不同固件下含义不一，不武断归类
		{"1", types.ValveStateCalibration, false},  // 无 A 前缀的纯数字也能解析
		{"2", types.ValveStateMeasurement, false},
	}
	for _, tc := range cases {
		t.Run(tc.resp, func(t *testing.T) {
			got, err := parseValveReadResponse(tc.resp)
			if tc.isErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.isErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestParseValveReadResponse_Textual 文本同义词映射（容错输入）
func TestParseValveReadResponse_Textual(t *testing.T) {
	cases := []struct {
		resp string
		want types.ValveState
	}{
		{"calibration", types.ValveStateCalibration},
		{"Calibrate", types.ValveStateCalibration},
		{"CALIBRATION", types.ValveStateCalibration},
		{"measurement", types.ValveStateMeasurement},
		{"Measure", types.ValveStateMeasurement},
	}
	for _, tc := range cases {
		t.Run(tc.resp, func(t *testing.T) {
			got, err := parseValveReadResponse(tc.resp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestParseValveReadResponse_Unknown 未识别响应归为 Unknown（不报错，便于 UI 继续操作）
func TestParseValveReadResponse_Unknown(t *testing.T) {
	cases := []string{"", "  ", "garbage", "XYZ", "12.5", "A5"}
	for _, resp := range cases {
		t.Run(resp, func(t *testing.T) {
			got, _ := parseValveReadResponse(resp)
			if got != types.ValveStateUnknown {
				t.Fatalf("got %v, want Unknown for resp %q", got, resp)
			}
		})
	}
}

// TestValveSetCommandFor 命令字映射
func TestValveSetCommandFor(t *testing.T) {
	cases := []struct {
		state types.ValveState
		want  string
		isErr bool
	}{
		{types.ValveStateCalibration, "w0C01", false},
		{types.ValveStateMeasurement, "w0C00", false},
		{types.ValveStateUnknown, "", true}, // Unknown 不可写入，必须报错
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			got, err := valveSetCommandFor(tc.state)
			if tc.isErr && err == nil {
				t.Fatalf("expected error for state %v", tc.state)
			}
			if !tc.isErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInterpretValveSetResponse_Success 成功响应 "A"
func TestInterpretValveSetResponse_Success(t *testing.T) {
	if err := interpretValveSetResponse("w0C01", "A"); err != nil {
		t.Fatalf("expected nil for success, got %v", err)
	}
	if err := interpretValveSetResponse("w0C00", " A "); err != nil {
		t.Fatalf("expected nil for trimmed success, got %v", err)
	}
}

// TestInterpretValveSetResponse_NACK 设备拒绝（如 N09）必须有明确错误
func TestInterpretValveSetResponse_NACK(t *testing.T) {
	err := interpretValveSetResponse("w0C01", "N09")
	if err == nil {
		t.Fatal("expected error for NACK response")
	}
}

// TestInterpretValveSetResponse_Unexpected 非预期响应报错
func TestInterpretValveSetResponse_Unexpected(t *testing.T) {
	cases := []string{"OK", "OK0", "1", ""}
	for _, resp := range cases {
		t.Run(resp, func(t *testing.T) {
			if err := interpretValveSetResponse("w0C01", resp); err == nil {
				t.Fatalf("expected error for resp %q", resp)
			}
		})
	}
}

// TestIsNACK Nxx 判定
func TestIsNACK(t *testing.T) {
	cases := []struct {
		resp string
		want bool
	}{
		{"N09", true},
		{"N03", true},
		{"N99", true},
		{"N0", true},      // 单数字也接受（保守判定为 NACK）
		{"N", false},      // 仅 N 无数字
		{"A", false},      // 成功响应
		{"OK", false},     // 其他
		{"N09X", false},   // 后跟非数字
		{"", false},       // 空
		{"n09", false},    // 小写不判定为 NACK（协议为大写）
	}
	for _, tc := range cases {
		t.Run(tc.resp, func(t *testing.T) {
			if got := isNACK(tc.resp); got != tc.want {
				t.Fatalf("isNACK(%q) = %v, want %v", tc.resp, got, tc.want)
			}
		})
	}
}
