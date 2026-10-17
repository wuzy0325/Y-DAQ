package five_hole

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yx-daq/internal/types"
)

func TestFiveHoleCsvWriter_HeaderAndDataPoint(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewFiveHoleCsvWriter()
	if err := w.Initialize(tmpDir, "test", "probe1", types.TraversalPatternLine, DefaultProbePrecision()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer w.Close()

	// 验证文件存在
	filePath := filepath.Join(tmpDir, "test_probe1.csv")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("csv file not created: %v", err)
	}

	// 写入数据点
	dp := types.FiveHoleTraversalDataPoint{
		PointID:       "pt-0",
		ProbeID:       "probe1",
		X:             10.5,
		Y:             20.3,
		XControllerName: "ctrl-α",
		XAxis:           "X",
		YControllerName: "ctrl-β",
		YAxis:           "Y",
		RawData: types.FiveHoleRawData{
			P1: 100.123456, P2: 101.654321, P3: 99.111111, P4: 100.222222, P5: 100.333333,
			PAtm: 101.325, TAtm: 20.5,
		},
		InterpResult: types.FiveHoleInterpolationResult{
			PtProbe:         102.5,
			PsProbe:         100.0,
			MachProbe:       0.45,
			AlphaProbe:      5.2,
			BetaProbe:       1.3,
			VelocityProbe:   150.0,
			CASProbe:        148.2,
			SATProbe:        293.15,
			DynamicPressure: 2500.0,
			Density:         1.225,
			VxProbe:         149.8,
			VyProbe:         5.0,
			VzProbe:         1.0,
			Valid:           true,
		},
		SampleCount: 10,
		Timestamp:   time.Date(2024, 1, 2, 3, 4, 5, 678000000, time.Local).UnixMilli(),
	}
	if err := w.AppendPoint(dp); err != nil {
		t.Fatalf("AppendPoint failed: %v", err)
	}

	// flush 确保写入
	w.Close()

	// 读取文件验证内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	str := string(content)

	// 验证 BOM
	if str[:3] != "\xEF\xBB\xBF" {
		t.Fatal("missing UTF-8 BOM")
	}

	// 验证表头含 β 列及富字段
	if !contains(str, "侧滑角Beta") {
		t.Fatal("header missing 侧滑角Beta")
	}
	if !contains(str, "X方向位移机构名") {
		t.Fatal("header missing X方向位移机构名")
	}
	if !contains(str, "Y方向位移机构名") {
		t.Fatal("header missing Y方向位移机构名")
	}
	if !contains(str, "P4") {
		t.Fatal("header missing P4")
	}
	if !contains(str, "P5") {
		t.Fatal("header missing P5")
	}
	if !contains(str, "校正空速CAS") {
		t.Fatal("header missing 校正空速CAS")
	}
	if !contains(str, "动压Qc") {
		t.Fatal("header missing 动压Qc")
	}
	if !contains(str, "速度Vz") {
		t.Fatal("header missing 速度Vz")
	}
	// 验证 T0（总温）列已移除
	if contains(str, "T0") {
		t.Fatal("header should not contain T0 (TTotal removed)")
	}
	// 验证旧字段已移除
	if contains(str, "迭代次数") {
		t.Fatal("header should not contain 迭代次数")
	}
	if contains(str, "收敛") {
		t.Fatal("header should not contain 收敛")
	}

	// 验证数据行
	if !contains(str, "probe1") {
		t.Fatal("data row missing probe1")
	}
	// 时间戳列为可读时间，且紧随点号列
	lines := strings.Split(strings.TrimRight(str, "\r\n"), "\n")
	headerFields := strings.Split(strings.TrimPrefix(lines[0], "\xEF\xBB\xBF"), ",")
	if len(headerFields) < 2 {
		t.Fatalf("header has too few columns: %v", headerFields)
	}
	if headerFields[0] != "点号" || headerFields[1] != "时间戳" {
		t.Fatalf("header should start with 点号,时间戳, got %s,%s", headerFields[0], headerFields[1])
	}
	if headerFields[len(headerFields)-1] != "采样数" {
		t.Fatalf("last header column should be 采样数, got %s", headerFields[len(headerFields)-1])
	}
	dataFields := strings.Split(lines[1], ",")
	if len(dataFields) != len(headerFields) {
		t.Fatalf("data field count %d != header field count %d", len(dataFields), len(headerFields))
	}
	if dataFields[0] != "pt-0" {
		t.Fatalf("first data column should be point id, got %s", dataFields[0])
	}
	if dataFields[1] != "2024-01-02 03:04:05.678" {
		t.Fatalf("timestamp should be readable, got %s", dataFields[1])
	}
	if !contains(str, "ctrl-α") {
		t.Fatal("data row missing alpha controller id")
	}
	if !contains(str, "ctrl-β") {
		t.Fatal("data row missing beta controller id")
	}
	if !contains(str, "5.20") {
		t.Fatal("data row missing alpha value (界面显示 2 位)")
	}
	if !contains(str, "1.30") {
		t.Fatal("data row missing beta value (界面显示 2 位)")
	}
	if !contains(str, "1.225000") {
		t.Fatal("data row missing density value (界面未显示，保持 6 位)")
	}
	if !contains(str, "100.123") {
		t.Fatal("data row missing P1 value (默认精度 3 位)")
	}
}

// TestFiveHoleCsvWriter_ProbePrecision 原始采集列按通道采集精度落盘（与界面显示一致）
func TestFiveHoleCsvWriter_ProbePrecision(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewFiveHoleCsvWriter()
	prec := ProbePrecision{P1: 1, P2: 2, P3: 0, P4: 4, P5: 3, PAtm: 2, TAtm: 1}
	if err := w.Initialize(tmpDir, "test", "probe1", types.TraversalPatternLine, prec); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	dp := types.FiveHoleTraversalDataPoint{
		PointID: "pt-0",
		ProbeID: "probe1",
		RawData: types.FiveHoleRawData{
			P1: 100.123456, P2: 101.654321, P3: 99.987654, P4: 100.222222, P5: 100.333333,
			PAtm: 101.346, TAtm: 20.55,
		},
		InterpResult: types.FiveHoleInterpolationResult{PtProbe: 102.5, MachProbe: 0.456789},
		SampleCount:  1,
		Timestamp:    time.Date(2024, 1, 2, 3, 4, 5, 678000000, time.Local).UnixMilli(),
	}
	if err := w.AppendPoint(dp); err != nil {
		t.Fatalf("AppendPoint failed: %v", err)
	}
	w.Close()

	content, err := os.ReadFile(filepath.Join(tmpDir, "test_probe1.csv"))
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(content), "\r\n"), "\n")
	fields := strings.Split(lines[1], ",")
	// 列序：点号,时间戳,探针ID,X,Y,机构,轴,机构,轴,P1..P5,P∞,T∞,Pt,...
	got := []string{fields[9], fields[10], fields[11], fields[12], fields[13], fields[14], fields[15]}
	want := []string{"100.1", "101.65", "100", "100.2222", "100.333", "101.35", "20.6"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 个采集列应为 %s，实际 %s", i+1, want[i], got[i])
		}
	}
	// Pt 界面 3 位、Ma 界面 4 位
	if fields[16] != "102.500" {
		t.Errorf("总压 Pt 应为 102.500（界面 3 位），实际 %s", fields[16])
	}
	if fields[18] != "0.4568" {
		t.Errorf("马赫数 Ma 应为 0.4568（界面 4 位），实际 %s", fields[18])
	}
}

// TestResolveProbePrecision 按探针通道配置/大气数据源解析落盘精度
func TestResolveProbePrecision(t *testing.T) {
	precisions := map[string]map[int]int{
		"devA": {0: 1, 1: 2, 2: 0, 3: 4, 4: 5},
		"devAtm": {0: 2, 1: 1},
	}
	get := func(deviceID string, channel int) (int, bool) {
		if m, ok := precisions[deviceID]; ok {
			if p, ok := m[channel]; ok {
				return p, true
			}
		}
		return 0, false
	}
	probe := types.FiveHoleProbeConfig{
		ProbeID: "probe1",
		ProbeChannels: []types.FiveHoleProbeChannelConfig{
			{Role: types.Role5H_P1, DeviceID: "devA", Channel: 0},
			{Role: types.Role5H_P2, DeviceID: "devA", Channel: 1},
			{Role: types.Role5H_P3, DeviceID: "devA", Channel: 2},
			{Role: types.Role5H_P4, DeviceID: "devA", Channel: 3},
			{Role: types.Role5H_P5, DeviceID: "devA", Channel: 4},
		},
		PAtmSource: types.FiveHoleAtmSource{Mode: types.FiveHoleSourceDevice, DeviceID: "devAtm", Channel: 0},
		TAtmSource: types.FiveHoleAtmSource{Mode: types.FiveHoleSourceManual, ManualValue: 20},
	}
	prec := ResolveProbePrecision(probe, get)
	want := ProbePrecision{P1: 1, P2: 2, P3: 0, P4: 4, P5: 5, PAtm: 2, TAtm: types.FallbackChannelPrecision}
	if prec != want {
		t.Errorf("解析结果应为 %+v，实际 %+v", want, prec)
	}

	// 未注入 getter / 通道未配置 → 全部回退默认精度
	def := DefaultProbePrecision()
	if got := ResolveProbePrecision(probe, nil); got != def {
		t.Errorf("未注入 getter 应回退默认精度 %+v，实际 %+v", def, got)
	}
	if got := ResolveProbePrecision(types.FiveHoleProbeConfig{}, get); got != def {
		t.Errorf("无通道配置应回退默认精度 %+v，实际 %+v", def, got)
	}
}

// TestResolveProbesPrecision_SkipsDisabled 仅解析启用探针
func TestResolveProbesPrecision_SkipsDisabled(t *testing.T) {
	probes := []types.FiveHoleProbeConfig{
		{ProbeID: "probe1", Enabled: true},
		{ProbeID: "probe2", Enabled: false},
	}
	got := ResolveProbesPrecision(probes, nil)
	if len(got) != 1 {
		t.Fatalf("应仅含启用探针，实际 %d 个", len(got))
	}
	if _, ok := got["probe1"]; !ok {
		t.Error("应包含 probe1")
	}
}

func TestFiveHoleCsvWriter_IndependentFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 两个探针各一个 writer
	w1 := NewFiveHoleCsvWriter()
	if err := w1.Initialize(tmpDir, "test", "probe1", types.TraversalPatternLine, DefaultProbePrecision()); err != nil {
		t.Fatalf("Initialize probe1 failed: %v", err)
	}
	defer w1.Close()

	w2 := NewFiveHoleCsvWriter()
	if err := w2.Initialize(tmpDir, "test", "probe2", types.TraversalPatternLine, DefaultProbePrecision()); err != nil {
		t.Fatalf("Initialize probe2 failed: %v", err)
	}
	defer w2.Close()

	// 验证两个独立文件
	if _, err := os.Stat(filepath.Join(tmpDir, "test_probe1.csv")); err != nil {
		t.Fatalf("probe1 csv not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "test_probe2.csv")); err != nil {
		t.Fatalf("probe2 csv not created: %v", err)
	}
}

func TestFiveHoleCsvWriter_FanHeader(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewFiveHoleCsvWriter()
	if err := w.Initialize(tmpDir, "test", "probe1", types.TraversalPatternFan, DefaultProbePrecision()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer w.Close()

	w.Close()
	content, err := os.ReadFile(filepath.Join(tmpDir, "test_probe1.csv"))
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	str := string(content)
	// 扇面模式点位列为 R(mm)/θ(°)，方向列标注 R/θ
	if !contains(str, "R(mm)") || !contains(str, "θ(°)") {
		t.Fatal("fan header missing R(mm)/θ(°) columns")
	}
	if !contains(str, "R方向位移机构名") || !contains(str, "θ方向位移机构名") {
		t.Fatal("fan header missing R/θ direction columns")
	}
	if contains(str, "X方向位移机构名") {
		t.Fatal("fan header should not contain X方向 columns")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
