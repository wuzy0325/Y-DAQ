package three_hole

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// ThreeHoleCsvWriter 三孔移位测试 CSV 写入器
// 并发说明：OnTestStart（主 goroutine）与 OnTestComplete（runTestLoop goroutine）
// 可能并发调用 Close/Initialize，必须用 mu 保护 file/writer/flushCnt
type ThreeHoleCsvWriter struct {
	mu       sync.Mutex
	file     *os.File
	writer   *csv.Writer
	flushCnt int // 累计写入点数，达到阈值时 flush
	// prec 原始采集列的落盘小数位（与界面显示一致，见 ResolveProbePrecision）
	prec ProbePrecision
}

const csvFlushInterval = 50 // 每写 50 点 flush 一次

// NewThreeHoleCsvWriter 创建 CSV 写入器
func NewThreeHoleCsvWriter() *ThreeHoleCsvWriter {
	return &ThreeHoleCsvWriter{}
}

// Initialize 初始化 CSV 文件（创建文件、写表头）
// prec: 原始采集列的落盘小数位（界面显示精度）
func (w *ThreeHoleCsvWriter) Initialize(savePath string, fileName string, prec ProbePrecision) error {
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
		slog.Error("write BOM to csv file failed", "err", err)
	}

	w.mu.Lock()
	w.file = file
	w.writer = csv.NewWriter(file)
	w.flushCnt = 0
	w.prec = prec
	w.mu.Unlock()

	// 写入表头（时间戳紧随点号；writer 已加锁保护，此处通过局部变量写入）
	w.mu.Lock()
	defer w.mu.Unlock()
	header := []string{
		"点号", "时间戳", "X", "Y",
		"P1", "P2", "P3", "P∞", "T∞",
		"总压Pt", "静压Ps", "马赫数Ma", "攻角Alpha", "速度V",
		"迭代次数", "采样数",
	}
	if err := w.writer.Write(header); err != nil {
		return fmt.Errorf("write header failed: %w", err)
	}
	w.writer.Flush()

	return nil
}

// AppendPoint 追加一个数据点
func (w *ThreeHoleCsvWriter) AppendPoint(dp types.ThreeHoleTraversalDataPoint) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer == nil {
		return fmt.Errorf("csv writer not initialized")
	}

	record := []string{
		dp.PointID,
		formatTimestamp(dp.Timestamp),
		formatPrec(dp.X, types.WritePrecPosition),
		formatPrec(dp.Y, types.WritePrecPosition),
		formatPrec(dp.RawData.P1, w.prec.P1),
		formatPrec(dp.RawData.P2, w.prec.P2),
		formatPrec(dp.RawData.P3, w.prec.P3),
		formatPrec(dp.RawData.PAtm, w.prec.PAtm),
		formatPrec(dp.RawData.TAtm, w.prec.TAtm),
		formatPrec(dp.InterpResult.PtProbe, types.WritePrecPt),
		formatPrec(dp.InterpResult.PsProbe, types.WritePrecPs),
		formatPrec(dp.InterpResult.MachProbe, types.WritePrecMach),
		formatPrec(dp.InterpResult.AlphaProbe, types.WritePrecAlpha),
		formatPrec(dp.InterpResult.VelocityProbe, types.WritePrecVelocity),
		fmt.Sprintf("%d", dp.InterpResult.IterationCount),
		fmt.Sprintf("%d", dp.SampleCount),
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}
}

// formatTimestamp 将 UnixMilli 时间戳格式化为可读时间（本地时区，毫秒精度）
// 与 realtime_recorder 的表头时间格式保持一致；时间戳缺失（<=0）时输出空字符串
func formatTimestamp(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05.000")
}
