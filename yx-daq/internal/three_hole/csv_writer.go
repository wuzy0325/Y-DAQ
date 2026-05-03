package three_hole

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/types"
)

// ThreeHoleCsvWriter 三孔移位测试 CSV 写入器
type ThreeHoleCsvWriter struct {
	file     *os.File
	writer   *csv.Writer
	flushCnt int // 累计写入点数，达到阈值时 flush
}

const csvFlushInterval = 50 // 每写 50 点 flush 一次

// NewThreeHoleCsvWriter 创建 CSV 写入器
func NewThreeHoleCsvWriter() *ThreeHoleCsvWriter {
	return &ThreeHoleCsvWriter{}
}

// Initialize 初始化 CSV 文件（创建文件、写表头）
func (w *ThreeHoleCsvWriter) Initialize(savePath string, fileName string) error {
	if fileName == "" {
		fileName = fmt.Sprintf("ThreeHoleTraversal-%s.csv", time.Now().Format("2006-01-02"))
	}
	if filepath.IsAbs(fileName) || !filepath.IsLocal(fileName) {
		return fmt.Errorf("invalid file name: %s", fileName)
	}
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

	// 写入表头
	header := []string{
		"点号", "X", "Y",
		"P1", "P2", "P3", "P∞", "T∞",
		"总压Pt", "静压Ps", "马赫数Ma", "攻角Alpha",
		"迭代次数", "采样数", "时间戳",
	}
	if err := w.writer.Write(header); err != nil {
		return fmt.Errorf("write header failed: %w", err)
	}
	w.writer.Flush()

	return nil
}

// AppendPoint 追加一个数据点
func (w *ThreeHoleCsvWriter) AppendPoint(dp types.ThreeHoleTraversalDataPoint) error {
	if w.writer == nil {
		return fmt.Errorf("csv writer not initialized")
	}

	record := []string{
		dp.PointID,
		fmt.Sprintf("%.4f", dp.X),
		fmt.Sprintf("%.4f", dp.Y),
		fmt.Sprintf("%.6f", dp.RawData.P1),
		fmt.Sprintf("%.6f", dp.RawData.P2),
		fmt.Sprintf("%.6f", dp.RawData.P3),
		fmt.Sprintf("%.6f", dp.RawData.PAtm),
		fmt.Sprintf("%.6f", dp.RawData.TAtm),
		fmt.Sprintf("%.6f", dp.InterpResult.PtProbe),
		fmt.Sprintf("%.6f", dp.InterpResult.PsProbe),
		fmt.Sprintf("%.6f", dp.InterpResult.MachProbe),
		fmt.Sprintf("%.4f", dp.InterpResult.AlphaProbe),
		fmt.Sprintf("%d", dp.InterpResult.IterationCount),
		fmt.Sprintf("%d", dp.SampleCount),
		fmt.Sprintf("%d", dp.Timestamp),
	}

	if err := w.writer.Write(record); err != nil {
		return fmt.Errorf("write record failed: %w", err)
	}

	w.flushCnt++
	if w.flushCnt%csvFlushInterval == 0 {
		w.writer.Flush()
	}

	return nil
}

// Close 关闭文件
func (w *ThreeHoleCsvWriter) Close() {
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}
}
