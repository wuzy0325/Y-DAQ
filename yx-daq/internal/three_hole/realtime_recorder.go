package three_hole

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// RealtimeRecorder 三孔实时数据录制器（保存实时刷新的原始压力 + 插值结果）
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
		"Timestamp", "PointID",
		"P1", "P2", "P3", "P∞", "T∞",
		"Pt", "Ps", "Ma", "α", "V",
		"Iterations",
	}
	if err := r.writer.Write(header); err != nil {
		r.recording.Store(false)
		file.Close()
		return fmt.Errorf("write header failed: %w", err)
	}
	r.writer.Flush()
	return nil
}

// Record 写入一条实时数据记录
func (r *RealtimeRecorder) Record(evt types.ThreeHoleTraversalRealtimeEvent) {
	if !r.recording.Load() {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording.Load() || r.writer == nil {
		return
	}

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	record := []string{
		ts,
		evt.PointID,
		fmt.Sprintf("%.6f", evt.RawData.P1),
		fmt.Sprintf("%.6f", evt.RawData.P2),
		fmt.Sprintf("%.6f", evt.RawData.P3),
		fmt.Sprintf("%.6f", evt.RawData.PAtm),
		fmt.Sprintf("%.2f", evt.RawData.TAtm),
		fmt.Sprintf("%.6f", evt.InterpResult.PtProbe),
		fmt.Sprintf("%.6f", evt.InterpResult.PsProbe),
		fmt.Sprintf("%.6f", evt.InterpResult.MachProbe),
		fmt.Sprintf("%.4f", evt.InterpResult.AlphaProbe),
		fmt.Sprintf("%.4f", evt.InterpResult.VelocityProbe),
		fmt.Sprintf("%d", evt.InterpResult.IterationCount),
	}
	r.writer.Write(record)
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
