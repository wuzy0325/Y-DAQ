package five_hole

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// RealtimeRecorder 五孔实时数据录制器
// 照三孔 RealtimeRecorder，适配五孔实时事件 types.FiveHoleTraversalRealtimeEvent：
// - 单个 CSV 文件，每次实时事件遍历 evt.ProbeRealtime 写入每探针一行
// - 表头含 Phase / ProbeID / P4 / P5 / β 列
type RealtimeRecorder struct {
	mu        sync.Mutex
	recording atomic.Bool
	file      *os.File
	writer    *csv.Writer
	// precisions 各探针原始采集列的落盘小数位（key=ProbeID）；缺失的探针用默认精度
	precisions map[string]ProbePrecision
}

// NewRealtimeRecorder 创建实时数据录制器
func NewRealtimeRecorder() *RealtimeRecorder {
	return &RealtimeRecorder{}
}

// Start 开始录制到指定文件路径（含完整文件名）
// precisions: 各探针原始采集列的落盘小数位（key=ProbeID，由 ResolveProbesPrecision 解析）；
// 传 nil 或探针缺失时该探针用默认精度（types.FallbackChannelPrecision）
func (r *RealtimeRecorder) Start(filePath string, precisions map[string]ProbePrecision) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording.Load() {
		return fmt.Errorf("already recording")
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create directory failed: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}

	// UTF-8 BOM
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		file.Close()
		return fmt.Errorf("write BOM failed: %w", err)
	}

	r.file = file
	r.writer = csv.NewWriter(file)
	r.precisions = precisions
	r.recording.Store(true)

	header := []string{
		"Timestamp", "PointID", "Phase", "ProbeID",
		"P1", "P2", "P3", "P4", "P5", "P∞", "T∞",
		"Pt", "Ps", "Ma", "α", "β", "V",
		"CAS", "SAT", "Qc", "ρ", "Vx", "Vy", "Vz",
	}
	if err := r.writer.Write(header); err != nil {
		r.recording.Store(false)
		file.Close()
		r.file = nil
		r.writer = nil
		return fmt.Errorf("write header failed: %w", err)
	}
	r.writer.Flush()
	return nil
}

// Record 写入一条实时数据记录
// 遍历 evt.ProbeRealtime，每探针写一行
func (r *RealtimeRecorder) Record(evt types.FiveHoleTraversalRealtimeEvent) {
	if !r.recording.Load() {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording.Load() || r.writer == nil {
		return
	}

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	for _, item := range evt.ProbeRealtime {
		prec, ok := r.precisions[item.ProbeID]
		if !ok {
			prec = DefaultProbePrecision()
		}
		record := []string{
			ts,
			evt.PointID,
			evt.Phase,
			item.ProbeID,
			formatPrec(item.RawData.P1, prec.P1),
			formatPrec(item.RawData.P2, prec.P2),
			formatPrec(item.RawData.P3, prec.P3),
			formatPrec(item.RawData.P4, prec.P4),
			formatPrec(item.RawData.P5, prec.P5),
			formatPrec(item.RawData.PAtm, prec.PAtm),
			formatPrec(item.RawData.TAtm, prec.TAtm),
			formatPrec(item.InterpResult.PtProbe, types.WritePrecPt),
			formatPrec(item.InterpResult.PsProbe, types.WritePrecPs),
			formatPrec(item.InterpResult.MachProbe, types.WritePrecMach),
			formatPrec(item.InterpResult.AlphaProbe, types.WritePrecAlpha),
			formatPrec(item.InterpResult.BetaProbe, types.WritePrecBeta),
			formatPrec(item.InterpResult.VelocityProbe, types.WritePrecVelocity),
			formatPrec(item.InterpResult.CASProbe, types.WritePrecCAS),
			formatPrec(item.InterpResult.SATProbe, types.WritePrecSAT),
			formatPrec(item.InterpResult.DynamicPressure, types.WritePrecQc),
			formatPrec(item.InterpResult.Density, types.WritePrecDensity),
			formatPrec(item.InterpResult.VxProbe, types.WritePrecVxyz),
			formatPrec(item.InterpResult.VyProbe, types.WritePrecVxyz),
			formatPrec(item.InterpResult.VzProbe, types.WritePrecVxyz),
		}
		if err := r.writer.Write(record); err != nil {
				slog.Debug("realtime recorder write failed", "err", err)
			}
	}
	r.writer.Flush()
}

// Stop 停止录制并关闭文件
func (r *RealtimeRecorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording.Load() {
		return
	}
	r.recording.Store(false)
	r.precisions = nil
	if r.writer != nil {
		r.writer.Flush()
		r.writer = nil
	}
	if r.file != nil {
		r.file.Close()
		r.file = nil
	}
}

// IsRecording 是否正在录制
func (r *RealtimeRecorder) IsRecording() bool {
	return r.recording.Load()
}
