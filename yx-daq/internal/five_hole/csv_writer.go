package five_hole

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/types"
)

// FiveHoleCsvWriter 五孔移位测试 CSV 写入器（每探针独立一个文件）
type FiveHoleCsvWriter struct {
	file     *os.File
	writer   *csv.Writer
	flushCnt int
}

const fiveHoleCsvFlushInterval = 50

// NewFiveHoleCsvWriter 创建 CSV 写入器
func NewFiveHoleCsvWriter() *FiveHoleCsvWriter {
	return &FiveHoleCsvWriter{}
}

// Initialize 初始化 CSV 文件（创建文件、写表头）
// savePath: 保存目录；baseName: 基础文件名（不含扩展名）；probeID: 探针ID（用于文件名区分）
func (w *FiveHoleCsvWriter) Initialize(savePath string, baseName string, probeID string) error {
	if baseName == "" {
		baseName = fmt.Sprintf("FiveHoleTraversal-%s", time.Now().Format("2006-01-02"))
	}
	// 文件名含 probeID 以区分各探针
	fileName := fmt.Sprintf("%s_%s.csv", baseName, probeID)

	if err := os.MkdirAll(savePath, 0755); err != nil {
		return fmt.Errorf("create save directory failed: %w", err)
	}
	filePath := filepath.Join(savePath, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create csv file failed: %w", err)
	}

	// 写入 UTF-8 BOM
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		log.Printf("write BOM to csv file failed: %v", err)
	}

	w.file = file
	w.writer = csv.NewWriter(file)

	// 写入表头（含 β 列）
	header := []string{
		"点号", "探针ID", "X", "Y",
		"P1", "P2", "P3", "P4", "P5", "P∞", "T∞",
		"总压Pt", "静压Ps", "马赫数Ma", "攻角Alpha", "侧滑角Beta", "速度V",
		"迭代次数", "收敛", "采样数", "时间戳",
	}
	if err := w.writer.Write(header); err != nil {
		return fmt.Errorf("write header failed: %w", err)
	}
	w.writer.Flush()

	return nil
}

// AppendPoint 追加一个数据点
func (w *FiveHoleCsvWriter) AppendPoint(dp types.FiveHoleTraversalDataPoint) error {
	if w.writer == nil {
		return fmt.Errorf("csv writer not initialized")
	}

	converged := "否"
	if dp.InterpResult.Converged {
		converged = "是"
	}

	record := []string{
		dp.PointID,
		dp.ProbeID,
		fmt.Sprintf("%.4f", dp.X),
		fmt.Sprintf("%.4f", dp.Y),
		fmt.Sprintf("%.6f", dp.RawData.P1),
		fmt.Sprintf("%.6f", dp.RawData.P2),
		fmt.Sprintf("%.6f", dp.RawData.P3),
		fmt.Sprintf("%.6f", dp.RawData.P4),
		fmt.Sprintf("%.6f", dp.RawData.P5),
		fmt.Sprintf("%.6f", dp.RawData.PAtm),
		fmt.Sprintf("%.6f", dp.RawData.TAtm),
		fmt.Sprintf("%.6f", dp.InterpResult.PtProbe),
		fmt.Sprintf("%.6f", dp.InterpResult.PsProbe),
		fmt.Sprintf("%.6f", dp.InterpResult.MachProbe),
		fmt.Sprintf("%.4f", dp.InterpResult.AlphaProbe),
		fmt.Sprintf("%.4f", dp.InterpResult.BetaProbe),
		fmt.Sprintf("%.4f", dp.InterpResult.VelocityProbe),
		fmt.Sprintf("%d", dp.InterpResult.IterationCount),
		converged,
		fmt.Sprintf("%d", dp.SampleCount),
		fmt.Sprintf("%d", dp.Timestamp),
	}

	if err := w.writer.Write(record); err != nil {
		return fmt.Errorf("write record failed: %w", err)
	}

	w.flushCnt++
	if w.flushCnt%fiveHoleCsvFlushInterval == 0 {
		w.writer.Flush()
	}

	return nil
}

// Close 关闭文件
func (w *FiveHoleCsvWriter) Close() {
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}
}
