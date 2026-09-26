package app

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"yx-daq/internal/logger"
	"yx-daq/internal/types"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// tailReadChunk 尾部读取的块大小
const tailReadChunk = 32 * 1024

// LogService 日志查看与配置服务
type LogService struct {
	Core *Core
}

// GetLogDir 获取日志目录路径
func (s *LogService) GetLogDir() string {
	return logger.LogDir()
}

// OpenLogDir 在资源管理器中打开日志目录
func (s *LogService) OpenLogDir() error {
	dir := logger.LogDir()
	if dir == "" {
		if err := logger.Init(types.DefaultLoggingConfig()); err != nil {
			return fmt.Errorf("日志系统未初始化: %w", err)
		}
		dir = logger.LogDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	if err := exec.Command("explorer.exe", dir).Start(); err != nil {
		return fmt.Errorf("打开日志目录失败: %w", err)
	}
	return nil
}

// ListLogFiles 列出日志文件（按修改时间从新到旧）
func (s *LogService) ListLogFiles() []types.LogFileInfo {
	dir := logger.LogDir()
	if dir == "" {
		return []types.LogFileInfo{}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []types.LogFileInfo{}
	}

	files := make([]types.LogFileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, types.LogFileInfo{
			Name:     e.Name(),
			Size:     info.Size(),
			Modified: info.ModTime().UnixMilli(),
			Category: types.CategoryFromLogName(e.Name()),
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Modified > files[j].Modified })
	return files
}

// ReadLogTail 读取日志文件末尾 maxLines 行（默认 200，上限 2000）
func (s *LogService) ReadLogTail(fileName string, maxLines int) (string, error) {
	if filepath.IsAbs(fileName) || !filepath.IsLocal(fileName) {
		return "", fmt.Errorf("非法日志文件名: %s", fileName)
	}
	if maxLines <= 0 {
		maxLines = 200
	}
	if maxLines > 2000 {
		maxLines = 2000
	}

	filePath := filepath.Join(logger.LogDir(), fileName)
	tail, err := readTailLines(filePath, maxLines)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("日志文件不存在: %s", fileName)
		}
		return "", fmt.Errorf("读取日志失败: %w", err)
	}
	return tail, nil
}

// ExportLogs 弹出保存对话框，将全部日志打包为 zip 导出
func (s *LogService) ExportLogs() (string, error) {
	dlg := s.Core.App.Dialog.SaveFile()
	dlg.SetOptions(&application.SaveFileDialogOptions{
		Title:    "导出日志",
		Filename: fmt.Sprintf("yx-daq-logs-%s.zip", time.Now().Format("2006-01-02-15-04-05")),
	})
	dlg.AddFilter("ZIP压缩包", "*.zip")
	filePath, err := dlg.PromptForSingleSelection()
	if err != nil {
		if isDialogCancelled(err) {
			return "", nil // 用户取消对话框，静默返回
		}
		return "", err
	}
	if filePath == "" {
		return "", nil
	}

	if err := logger.ExportZip(filePath); err != nil {
		return "", fmt.Errorf("导出日志失败: %w", err)
	}
	return filePath, nil
}

// ClearLogs 清空全部日志文件
func (s *LogService) ClearLogs() error {
	if err := logger.ClearLogs(); err != nil {
		return fmt.Errorf("清空日志失败: %w", err)
	}
	return nil
}

// GetLoggingConfig 获取当前日志配置
func (s *LogService) GetLoggingConfig() types.LoggingConfig {
	return logger.GetConfig()
}

// SetLoggingConfig 保存并应用日志配置（级别/控制台/通信日志/保留与滚动策略）
func (s *LogService) SetLoggingConfig(cfg types.LoggingConfig) error {
	if !logger.ValidLevel(cfg.Level) {
		return fmt.Errorf("非法日志级别: %s", cfg.Level)
	}
	cfg = cfg.Normalize()
	if err := s.Core.ConfigManager.Logging.Set(cfg); err != nil {
		return fmt.Errorf("保存日志配置失败: %w", err)
	}
	logger.Configure(cfg)
	slog.Info("logging config updated", "level", cfg.Level, "comm_enabled", cfg.CommEnabled, "frontend_errors", cfg.FrontendErrors)
	return nil
}

// WriteFrontendError 记录前端 JS 错误（Vue errorHandler / window.onerror 上报）
func (s *LogService) WriteFrontendError(level string, message string, stack string) {
	if s.Core.ConfigManager != nil && !s.Core.ConfigManager.Logging.Get().FrontendErrors {
		return
	}
	if message == "" && stack == "" {
		return
	}

	attrs := []any{
		"source", "frontend",
		"message", logger.Truncate(message, 2000),
	}
	if stack != "" {
		attrs = append(attrs, "stack", logger.Truncate(stack, 4000))
	}

	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slog.Debug("frontend error", attrs...)
	case "warn", "warning":
		slog.Warn("frontend error", attrs...)
	case "info":
		slog.Info("frontend error", attrs...)
	default:
		slog.Error("frontend error", attrs...)
	}
}

// readTailLines 读取文件末尾 maxLines 行（从文件尾部按块反向读取，避免整文件载入内存）
func readTailLines(path string, maxLines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()
	if size == 0 {
		return "", nil
	}

	var data []byte
	pos := size
	for pos > 0 {
		readSize := int64(tailReadChunk)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize
		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, pos); err != nil && err != io.EOF {
			return "", err
		}
		data = append(buf, data...)
		if bytes.Count(data, []byte{'\n'}) > maxLines {
			break
		}
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n"), nil
}
