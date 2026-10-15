package driver

import "testing"

// TestParseADAveragingResponse 验证 q05 响应解析（十六进制、A 前缀、NACK、非法）
func TestParseADAveragingResponse(t *testing.T) {
	cases := []struct {
		name    string
		resp    string
		want    int
		wantErr bool
	}{
		{"四位数十六进制", "000A", 10, false},
		{"A 前缀", "A000A", 10, false},
		{"裸十六进制", "40", 64, false},
		{"首位为A的裸十六进制", "A0", 160, false},
		{"最大合法值", "FF", 255, false},
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

// TestADAveragingWriteCmd 固定 w 命令线格式：索引 05 + 2 位十六进制值。
// 真机若期望 4 位定宽值，此测试会失败并提示需确认文档 3.6 示例（w100A）。
func TestADAveragingWriteCmd(t *testing.T) {
	cases := []struct {
		count int
		want  string
	}{
		{10, "w050A"},
		{4, "w0504"},
		{8, "w0508"},
		{255, "w05FF"},
	}
	for _, tc := range cases {
		if got := adAvgWriteCmd(tc.count); got != tc.want {
			t.Errorf("adAvgWriteCmd(%d) = %q, want %q", tc.count, got, tc.want)
		}
	}
}
