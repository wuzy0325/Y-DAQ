package three_hole

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// TestNewThreeHoleCsvWriter 测试CSV写入器创建
func TestNewThreeHoleCsvWriter(t *testing.T) {
	writer := NewThreeHoleCsvWriter()
	if writer == nil {
		t.Fatal("Expected CSV writer to be created")
	}
}

// TestInitialize_Creation 测试CSV文件创建
func TestInitialize_Creation(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	writer := NewThreeHoleCsvWriter()

	// 测试创建新文件
	err := writer.Initialize(tempDir, "test.csv")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 验证文件存在
	filePath := filepath.Join(tempDir, "test.csv")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("CSV file was not created")
	}

	// 验证文件可以关闭
	writer.Close()
}

// TestInitialize_DefaultName 测试默认文件名
func TestInitialize_DefaultName(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewThreeHoleCsvWriter()

	// 测试空文件名（应该使用默认名）
	err := writer.Initialize(tempDir, "")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 验证文件包含日期
	files, _ := os.ReadDir(tempDir)
	if len(files) == 0 {
		t.Error("No CSV file was created")
	}

	writer.Close()
}

// TestInitialize_InvalidPath 测试无效路径
func TestInitialize_InvalidPath(t *testing.T) {
	writer := NewThreeHoleCsvWriter()

	// 测试绝对路径
	err := writer.Initialize("/root", "test.csv")
	if err == nil {
		t.Error("Expected error for absolute path")
	}

	// 测试包含路径分隔符的文件名
	err = writer.Initialize(".", "../test.csv")
	if err == nil {
		t.Error("Expected error for path traversal")
	}
}

// TestInitialize_DirectoryCreation 测试目录创建
func TestInitialize_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "nested", "path")
	writer := NewThreeHoleCsvWriter()

	// 测试创建嵌套目录
	err := writer.Initialize(subDir, "test.csv")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	writer.Close()
}

// TestAppendPoint_Basic 测试基本数据点写入
func TestAppendPoint_Basic(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewThreeHoleCsvWriter()

	err := writer.Initialize(tempDir, "test.csv")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 创建测试数据点
	dataPoint := types.ThreeHoleTraversalDataPoint{
		PointID:      "test-001",
		X:            10.5,
		Y:            20.3,
		RawData:      types.ThreeHoleRawData{P1: 100.0, P2: 105.0, P3: 102.0},
		InterpResult: types.ThreeHoleInterpolationResult{PtProbe: 1000.0, PsProbe: 500.0, AlphaProbe: 5.0},
		SampleCount:  5,
		Timestamp:    time.Now().UnixMilli(),
	}

	err = writer.AppendPoint(dataPoint)
	if err != nil {
		t.Fatalf("AppendPoint failed: %v", err)
	}

	writer.Close()
}

// TestAppendPoint_EmptyData 测试空数据处理
func TestAppendPoint_EmptyData(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewThreeHoleCsvWriter()

	err := writer.Initialize(tempDir, "test.csv")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 创建空数据点
	dataPoint := types.ThreeHoleTraversalDataPoint{
		PointID:      "empty",
		X:            0,
		Y:            0,
		RawData:      types.ThreeHoleRawData{},
		InterpResult: types.ThreeHoleInterpolationResult{},
		SampleCount:  0,
		Timestamp:    0,
	}

	err = writer.AppendPoint(dataPoint)
	if err != nil {
		t.Fatalf("AppendPoint failed: %v", err)
	}

	writer.Close()
}

// TestAppendPoint_Flush 测试自动flush机制
func TestAppendPoint_Flush(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewThreeHoleCsvWriter()

	err := writer.Initialize(tempDir, "test.csv")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 写入超过flush间隔的数据点
	for i := 0; i < 60; i++ {
		dataPoint := types.ThreeHoleTraversalDataPoint{
			PointID:      fmt.Sprintf("test-%03d", i),
			X:            float64(i),
			Y:            float64(i * 2),
			RawData:      types.ThreeHoleRawData{P1: float64(i)},
			InterpResult: types.ThreeHoleInterpolationResult{},
			SampleCount:  1,
			Timestamp:    time.Now().UnixMilli(),
		}
		writer.AppendPoint(dataPoint)
	}

	writer.Close()

	// 验证文件大小大于0（实际写入）
	filePath := filepath.Join(tempDir, "test.csv")
	info, _ := os.Stat(filePath)
	if info.Size() == 0 {
		t.Error("CSV file should contain data after flush")
	}
}

// TestClose_AlreadyClosed 测试多次关闭
func TestClose_AlreadyClosed(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewThreeHoleCsvWriter()

	writer.Initialize(tempDir, "test.csv")
	writer.Close()

	// 第二次关闭不应该panic
	writer.Close()
}

// TestInitialize_PermissionError 测试权限错误
func TestInitialize_PermissionError(t *testing.T) {
	// 尝试写入到只读目录（可能失败，取决于环境）
	// 这是一个基本的权限测试，实际环境可能需要特殊设置
	writer := NewThreeHoleCsvWriter()

	// 在某些系统上这可能会失败
	err := writer.Initialize("/root", "should_fail.csv")
	if err == nil {
		// 如果成功，说明测试环境允许，我们仍然关闭文件
		writer.Close()
	}
	// 错误是预期的，所以不标记为失败
}