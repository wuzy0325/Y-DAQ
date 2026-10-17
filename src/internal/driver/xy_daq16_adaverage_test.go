package driver

import "testing"

// TestParseADAveragingResponse 验证 q05 响应解析（十六进制、ACK 粘连/残留、NACK、非法）
func TestParseADAveragingResponse(t *testing.T) {
	cases := []struct {
		name    string
		resp    string
		want    int
		wantErr bool
	}{
		{"四位数十六进制", "000A", 10, false},
		{"实测AD=4", "0004", 4, false},
		{"实测AD=16", "0010", 16, false},
		{"写ACK与响应粘连", "A000A", 10, false},
		{"裸十六进制", "40", 64, false},
		{"首位为A的裸十六进制", "A0", 160, false},
		{"最大合法值", "FF", 255, false},
		{"裸ACK残留", "A", 0, true},
		{"裸ACK残留小写", "a", 0, true},
		{"设备拒绝", "N07", 0, true},
		{"空响应", "", 0, true},
		{"非法字符", "ZZ", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseADAveragingResponse(tc.resp)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseADAveragingResponse(%q) expected error, got %d", tc.resp, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseADAveragingResponse(%q) unexpected error: %v", tc.resp, err)
			}
			if got != tc.want {
				t.Errorf("parseADAveragingResponse(%q) = %d, want %d", tc.resp, got, tc.want)
			}
		})
	}
}

// TestADAveragingWriteCmd 固定 w 命令线格式：索引 10 + 2 位十六进制值。
// 真机实测：AD=4 → w1004，AD=8 → w1008，AD=16 → w1010，AD=32 → w1020；
// 多写高位（w101004 之类）会被设备按 w10+10 解析，虽然回 A 但设置不生效。
func TestADAveragingWriteCmd(t *testing.T) {
	cases := []struct {
		count int
		want  string
	}{
		{4, "w1004"},
		{8, "w1008"},
		{16, "w1010"},
		{32, "w1020"},
		{10, "w100A"}, // 与硬件文档 3.6 示例一致
		{255, "w10FF"},
	}
	for _, tc := range cases {
		if got := adAvgWriteCmd(tc.count); got != tc.want {
			t.Errorf("adAvgWriteCmd(%d) = %q, want %q", tc.count, got, tc.want)
		}
	}
}

// TestInterpretADAveragingWriteResponse 写入应答校验：真机实测成功回 "A"，拒绝回 Nxx；
// 其余应答一律判为协议异常（不静默通过）
func TestInterpretADAveragingWriteResponse(t *testing.T) {
	cases := []struct {
		resp    string
		wantErr bool
	}{
		{"A", false},
		{" A \r\n", false},
		{"", true},      // 无应答不能当成功
		{"A0008", true}, // 非预期形态
		{"OK", true},
		{"N07", true},
		{" N09 ", true},
	}
	for _, tc := range cases {
		err := interpretADAveragingWriteResponse(tc.resp)
		if tc.wantErr {
			if err == nil {
				t.Errorf("interpretADAveragingWriteResponse(%q) expected error", tc.resp)
			}
			continue
		}
		if err != nil {
			t.Errorf("interpretADAveragingWriteResponse(%q) unexpected error: %v", tc.resp, err)
		}
	}
}
