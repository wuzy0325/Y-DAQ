package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/types"

	"github.com/go-pdf/fpdf"
)

// PdfReportService PDF报告生成服务
type PdfReportService struct{}

// NewPdfReportService 创建PDF报告服务
func NewPdfReportService() *PdfReportService {
	return &PdfReportService{}
}

// ExportCalibrationReport 导出校准PDF报告
func (s *PdfReportService) ExportCalibrationReport(
	dataPoints []types.CalibrationDataPoint,
	config types.CalibrationConfig,
	outputPath string,
) error {
	os.MkdirAll(filepath.Dir(outputPath), 0755) // ignore error: 目录已存在或后续pdf.OutputFileAndClose会报错

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// 注册中文字体 (使用内置Helvetica，中文可能显示为方块，但基本ASCII内容正常)
	// 对于完整中文支持，需要嵌入字体文件，此处使用内置字体保证基本可用
	pdf.SetFont("Helvetica", "B", 20)

	// 标题
	pdf.SetTextColor(40, 40, 120)
	pdf.CellFormat(0, 12, "YX-DAQ Five-Hole Probe Calibration Report", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// 分隔线
	pdf.SetDrawColor(100, 100, 200)
	pdf.SetLineWidth(0.5)
	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
	pdf.Ln(6)

	// 报告信息
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)
	now := time.Now().Format("2006-01-02 15:04:05")
	s.infoRow(pdf, "Report Date:", now)
	s.infoRow(pdf, "Calibration Type:", string(config.Type))
	s.infoRow(pdf, "Device ID:", config.DeviceID)
	s.infoRow(pdf, "Controller ID:", config.ControllerID)
	s.infoRow(pdf, "Alpha Axis:", string(config.AlphaAxis))
	s.infoRow(pdf, "Beta Axis:", string(config.BetaAxis))
	s.infoRow(pdf, "Dwell Time:", fmt.Sprintf("%d ms", config.DwellTimeMs))
	s.infoRow(pdf, "Samples Per Point:", fmt.Sprintf("%d", config.SamplesPerPoint))
	s.infoRow(pdf, "Total Points:", fmt.Sprintf("%d", len(dataPoints)))
	pdf.Ln(4)

	// 探针通道配置
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(40, 40, 120)
	pdf.CellFormat(0, 8, "Probe Channel Configuration", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)
	for _, ch := range config.ProbeChannels {
		if ch.Enabled {
			s.infoRow(pdf, fmt.Sprintf("  %s (%s):", ch.Name, string(ch.Role)), fmt.Sprintf("Channel %d", ch.Channel))
		}
	}
	pdf.Ln(4)

	// 校准结果表格
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(40, 40, 120)
	pdf.CellFormat(0, 8, "Calibration Results", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	if len(dataPoints) > 0 {
		s.drawDataTable(pdf, dataPoints)
	} else {
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 8, "No calibration data available.", "", 1, "L", false, 0, "")
	}

	// 统计摘要
	if len(dataPoints) > 0 {
		pdf.Ln(6)
		s.drawSummary(pdf, dataPoints)
	}

	// 页脚
	pdf.SetY(-15)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(150, 150, 150)
	pdf.CellFormat(0, 10, fmt.Sprintf("YX-DAQ Calibration Report - Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")

	return pdf.OutputFileAndClose(outputPath)
}

// infoRow 写入一行信息 (label: value)
func (s *PdfReportService) infoRow(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(50, 6, label, "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, value, "", 1, "L", false, 0, "")
}

// drawDataTable 绘制校准数据表格
func (s *PdfReportService) drawDataTable(pdf *fpdf.Fpdf, dataPoints []types.CalibrationDataPoint) {
	// 表头
	headers := []string{"No.", "Alpha", "Beta", "P1", "P2", "P3", "P4", "P5", "Ka", "Kb", "CPT", "CPS", "N", "StdDev"}
	colWidths := []float64{8, 14, 14, 16, 16, 16, 16, 16, 16, 16, 16, 16, 10, 16}

	// 表头背景
	pdf.SetFillColor(60, 60, 140)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 7)

	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// 数据行
	pdf.SetFont("Helvetica", "", 7)
	for rowIdx, dp := range dataPoints {
		// 检查是否需要新页
		if pdf.GetY() > 270 {
			pdf.AddPage()
			// 重绘表头
			pdf.SetFillColor(60, 60, 140)
			pdf.SetTextColor(255, 255, 255)
			pdf.SetFont("Helvetica", "B", 7)
			for i, h := range headers {
				pdf.CellFormat(colWidths[i], 6, h, "1", 0, "C", true, 0, "")
			}
			pdf.Ln(-1)
			pdf.SetFont("Helvetica", "", 7)
		}

		// 交替行背景
		if rowIdx%2 == 0 {
			pdf.SetFillColor(240, 240, 255)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(30, 30, 30)

		pTotalStr := ""
		if dp.RawData.PTotal != nil {
			pTotalStr = fmt.Sprintf("%.2f", *dp.RawData.PTotal)
		}
		_ = pTotalStr // PTotal暂不显示在表格中

		cells := []string{
			fmt.Sprintf("%d", rowIdx+1),
			fmt.Sprintf("%.2f", dp.Alpha),
			fmt.Sprintf("%.2f", dp.Beta),
			fmt.Sprintf("%.2f", dp.RawData.P1),
			fmt.Sprintf("%.2f", dp.RawData.P2),
			fmt.Sprintf("%.2f", dp.RawData.P3),
			fmt.Sprintf("%.2f", dp.RawData.P4),
			fmt.Sprintf("%.2f", dp.RawData.P5),
			fmt.Sprintf("%.4f", dp.Coefficients.Kalpha),
			fmt.Sprintf("%.4f", dp.Coefficients.Kbeta),
			fmt.Sprintf("%.4f", dp.Coefficients.CPT),
			fmt.Sprintf("%.4f", dp.Coefficients.CPS),
			fmt.Sprintf("%d", dp.SampleCount),
			fmt.Sprintf("%.4f", dp.StdDev),
		}

		for i, c := range cells {
			pdf.CellFormat(colWidths[i], 5, c, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}
}

// drawSummary 绘制统计摘要
func (s *PdfReportService) drawSummary(pdf *fpdf.Fpdf, dataPoints []types.CalibrationDataPoint) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(40, 40, 120)
	pdf.CellFormat(0, 8, "Statistical Summary", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// 计算各系数的统计量
	n := len(dataPoints)
	sumKa, sumKb, sumCPT, sumCPS := 0.0, 0.0, 0.0, 0.0
	sumStdDev := 0.0
	for _, dp := range dataPoints {
		sumKa += dp.Coefficients.Kalpha
		sumKb += dp.Coefficients.Kbeta
		sumCPT += dp.Coefficients.CPT
		sumCPS += dp.Coefficients.CPS
		sumStdDev += dp.StdDev
	}

	avgKa := sumKa / float64(n)
	avgKb := sumKb / float64(n)
	avgCPT := sumCPT / float64(n)
	avgCPS := sumCPS / float64(n)
	avgStdDev := sumStdDev / float64(n)

	// 标准差
	varKa, varKb, varCPT, varCPS := 0.0, 0.0, 0.0, 0.0
	for _, dp := range dataPoints {
		varKa += (dp.Coefficients.Kalpha - avgKa) * (dp.Coefficients.Kalpha - avgKa)
		varKb += (dp.Coefficients.Kbeta - avgKb) * (dp.Coefficients.Kbeta - avgKb)
		varCPT += (dp.Coefficients.CPT - avgCPT) * (dp.Coefficients.CPT - avgCPT)
		varCPS += (dp.Coefficients.CPS - avgCPS) * (dp.Coefficients.CPS - avgCPS)
	}
	stdKa := sqrt(varKa / float64(n))
	stdKb := sqrt(varKb / float64(n))
	stdCPT := sqrt(varCPT / float64(n))
	stdCPS := sqrt(varCPS / float64(n))

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)

	summaryHeaders := []string{"Coefficient", "Mean", "Std Dev", "Min", "Max"}
	summaryWidths := []float64{30, 30, 30, 30, 30}

	pdf.SetFillColor(60, 60, 140)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 9)
	for i, h := range summaryHeaders {
		pdf.CellFormat(summaryWidths[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(30, 30, 30)

	// 计算min/max
	minKa, maxKa := dataPoints[0].Coefficients.Kalpha, dataPoints[0].Coefficients.Kalpha
	minKb, maxKb := dataPoints[0].Coefficients.Kbeta, dataPoints[0].Coefficients.Kbeta
	minCPT, maxCPT := dataPoints[0].Coefficients.CPT, dataPoints[0].Coefficients.CPT
	minCPS, maxCPS := dataPoints[0].Coefficients.CPS, dataPoints[0].Coefficients.CPS
	for _, dp := range dataPoints {
		minKa, maxKa = fmin(minKa, dp.Coefficients.Kalpha), fmax(maxKa, dp.Coefficients.Kalpha)
		minKb, maxKb = fmin(minKb, dp.Coefficients.Kbeta), fmax(maxKb, dp.Coefficients.Kbeta)
		minCPT, maxCPT = fmin(minCPT, dp.Coefficients.CPT), fmax(maxCPT, dp.Coefficients.CPT)
		minCPS, maxCPS = fmin(minCPS, dp.Coefficients.CPS), fmax(maxCPS, dp.Coefficients.CPS)
	}

	summaryRows := [][]string{
		{"Kalpha", fmt.Sprintf("%.4f", avgKa), fmt.Sprintf("%.4f", stdKa), fmt.Sprintf("%.4f", minKa), fmt.Sprintf("%.4f", maxKa)},
		{"Kbeta", fmt.Sprintf("%.4f", avgKb), fmt.Sprintf("%.4f", stdKb), fmt.Sprintf("%.4f", minKb), fmt.Sprintf("%.4f", maxKb)},
		{"CPT", fmt.Sprintf("%.4f", avgCPT), fmt.Sprintf("%.4f", stdCPT), fmt.Sprintf("%.4f", minCPT), fmt.Sprintf("%.4f", maxCPT)},
		{"CPS", fmt.Sprintf("%.4f", avgCPS), fmt.Sprintf("%.4f", stdCPS), fmt.Sprintf("%.4f", minCPS), fmt.Sprintf("%.4f", maxCPS)},
	}

	for rowIdx, row := range summaryRows {
		if rowIdx%2 == 0 {
			pdf.SetFillColor(240, 240, 255)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for i, c := range row {
			pdf.CellFormat(summaryWidths[i], 5, c, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)
	s.infoRow(pdf, "Average Std Dev:", fmt.Sprintf("%.6f", avgStdDev))
}

func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	// Newton's method
	z := 1.0
	for i := 0; i < 20; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

func fmin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func fmax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
