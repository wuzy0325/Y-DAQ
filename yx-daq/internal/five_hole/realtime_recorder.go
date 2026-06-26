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
}

// NewRealtimeRecorder 创建实时数据录制器
func NewRealtimeRecorder() *RealtimeRecorder {
	return &RealtimeRecorder{}
}

// Start 开始录制到指定文件路径（含完整文件名）
func (r *RealtimeRecorder) Start(filePath string) error {
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
	r.recording.Store(true)

	header := []string{
		"Timestamp", "PointID", "Phase", "ProbeID",
		"P1", "P2", "P3", "P4", "P5", "P∞", "T∞",
		"Pt", "Ps", "Ma", "α", "β", "V",
		"Iterations",
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
		record := []string{
			ts,
			evt.PointID,
			evt.Phase,
			item.ProbeID,
			fmt.Sprintf("%.6f", item.RawData.P1),
			fmt.Sprintf("%.6f", item.RawData.P2),
			fmt.Sprintf("%.6f", item.RawData.P3),
			fmt.Sprintf("%.6f", item.RawData.P4),
			fmt.Sprintf("%.6f", item.RawData.P5),
			fmt.Sprintf("%.6f", item.RawData.PAtm),
			fmt.Sprintf("%.2f", item.RawData.TAtm),
			fmt.Sprintf("%.6f", item.InterpResult.PtProbe),
			fmt.Sprintf("%.6f", item.InterpResult.PsProbe),
			fmt.Sprintf("%.6f", item.InterpResult.MachProbe),
			fmt.Sprintf("%.4f", item.InterpResult.AlphaProbe),
			fmt.Sprintf("%.4f", item.InterpResult.BetaProbe),
			fmt.Sprintf("%.4f", item.InterpResult.VelocityProbe),
			fmt.Sprintf("%d", item.InterpResult.IterationCount),
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
