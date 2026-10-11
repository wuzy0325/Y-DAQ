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
