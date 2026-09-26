package logger

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

const appName = "yx-daq"

var (
	mu       sync.Mutex
	logDir   string
	levelVar = new(slog.LevelVar)
	started  bool

	// writers 持有当前全部滚动 writer（InitAt 重复调用时先关闭旧实例）
	writers []*rotateWriter
	appW    *rotateWriter

	// consoleOn 控制台输出开关（运行时切换，无需重建 handler）
	consoleOn atomic.Bool
	// commOn 设备通信日志开关
	commOn atomic.Bool

	commLogger  atomic.Pointer[slog.Logger]
	crashLogger atomic.Pointer[slog.Logger]

	current = types.DefaultLoggingConfig()
)

// switchWriter 按开关决定是否透传写入（用于控制台输出动态开关）
type switchWriter struct {
	on *atomic.Bool
	w  io.Writer
}

func (s switchWriter) Write(p []byte) (int, error) {
	if !s.on.Load() {
		return len(p), nil
	}
	return s.w.Write(p)
}

// fanoutWriter 依次写入所有目标，单个目标失败不阻止其余目标。
// 取代 io.MultiWriter：MultiWriter 在首个 writer 出错时立即返回，
// 会连带抑制后续（如控制台）输出，磁盘满/日志目录失效时用户将看不到任何日志。
type fanoutWriter []io.Writer

func (f fanoutWriter) Write(p []byte) (int, error) {
	var firstErr error
	for _, w := range f {
		if _, err := w.Write(p); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return len(p), firstErr
}

// Init 使用默认目录（~/.yx-daq/logs）初始化日志系统。
// 日志固定写入 ~/.yx-daq/logs/yx-daq-YYYY-MM-DD.log，保留配置天数；
// 使用用户主目录避免 Program Files 等 UAC 虚拟化目录导致写入失效
// （非管理员运行时对 Program Files 的写入会被透明重定向到 VirtualStore，
// 导致目标路径下的文件成为 0 字节空壳）。
func Init(cfg types.LoggingConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get user home dir: %w", err)
	}
	return InitAt(filepath.Join(home, ".yx-daq", "logs"), cfg)
}

// InitAt 在指定目录初始化日志系统（便于测试）。
// 初始化失败时调用方应继续运行：未初始化的 CommError/Recover 为安全空操作，
// slog 保持默认 handler 输出到 stderr。
func InitAt(dir string, cfg types.LoggingConfig) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	closeWritersLocked()
	logDir = dir

	cfg = cfg.Normalize()
	current = cfg
	levelVar.Set(ParseLevel(cfg.Level))
	consoleOn.Store(cfg.Console)
	commOn.Store(cfg.CommEnabled)

	cleanOldLogs(dir, cfg.RetentionDays)

	appW = newRotateWriter(dir, appName, cfg.MaxFileSizeMB)
	commW := newRotateWriter(dir, "comm", cfg.MaxFileSizeMB)
	crashW := newRotateWriter(dir, "crash", cfg.MaxFileSizeMB)
	writers = []*rotateWriter{appW, commW, crashW}

	handlerOpts := &slog.HandlerOptions{Level: levelVar}
	slog.SetDefault(slog.New(slog.NewJSONHandler(fanoutWriter{appW, switchWriter{&consoleOn, os.Stderr}}, handlerOpts)))
	commLogger.Store(slog.New(slog.NewJSONHandler(fanoutWriter{commW, switchWriter{&consoleOn, os.Stderr}}, handlerOpts)))
	crashLogger.Store(slog.New(slog.NewTextHandler(fanoutWriter{crashW, switchWriter{&consoleOn, os.Stderr}}, handlerOpts)))

	started = true
	slog.Info("logger initialized",
		"dir", dir,
		"level", cfg.Level,
		"retention_days", cfg.RetentionDays,
		"max_file_size_mb", cfg.MaxFileSizeMB,
	)
	return nil
}

// Configure 运行时应用日志配置（级别/控制台/通信日志/保留与滚动策略）。
// 未初始化或参数非法时安全忽略，由调用方（ConfigService/LogService）负责持久化与校验。
func Configure(cfg types.LoggingConfig) {
	mu.Lock()
	defer mu.Unlock()
	if !started {
		return
	}

	cfg = cfg.Normalize()
	levelVar.Set(ParseLevel(cfg.Level))
	consoleOn.Store(cfg.Console)
	commOn.Store(cfg.CommEnabled)
	for _, w := range writers {
		w.SetMaxSizeMB(cfg.MaxFileSizeMB)
	}
	current = cfg
}

// GetConfig 返回当前生效的日志配置
func GetConfig() types.LoggingConfig {
	mu.Lock()
	defer mu.Unlock()
	return current
}

// LogDir 返回日志目录（日志未初始化时为空字符串）
func LogDir() string {
	mu.Lock()
	defer mu.Unlock()
	return logDir
}

// ClearLogs 关闭并删除全部日志文件，后续写入自动重建文件。
// 注意：Windows 下必须先关闭句柄再删除，否则会因文件占用失败。
func ClearLogs() error {
	mu.Lock()
	defer mu.Unlock()
	if !started {
		return nil
	}

	for _, w := range writers {
		w.Close()
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("read log dir: %w", err)
	}
	var firstErr error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		if err := os.Remove(filepath.Join(logDir, e.Name())); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("remove %s: %w", e.Name(), err)
		}
	}
	return firstErr
}

// ExportZip 将全部日志文件打包为 zip 写入 dst（导出时跳过无法读取的文件）
func ExportZip(dst string) error {
	mu.Lock()
	defer mu.Unlock()
	if logDir == "" {
		return fmt.Errorf("logger not initialized")
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("read log dir: %w", err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		src, err := os.Open(filepath.Join(logDir, e.Name()))
		if err != nil {
			continue
		}
		w, err := zw.Create(e.Name())
		if err != nil {
			src.Close()
			return fmt.Errorf("create zip entry: %w", err)
		}
		if _, err := io.Copy(w, src); err != nil {
			src.Close()
			return fmt.Errorf("copy log %s: %w", e.Name(), err)
		}
		src.Close()
	}

	return nil
}

// ParseLevel 解析日志级别字符串（debug/info/warn/error），非法值回退 info
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ValidLevel 判断日志级别字符串是否合法
func ValidLevel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "info", "warn", "warning", "error":
		return true
	default:
		return false
	}
}

// Truncate 按字符截断过长文本（命令/响应日志使用，防止日志被单个超长内容撑爆）
func Truncate(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "...(truncated)"
}

// Close 关闭日志文件（应用退出时调用）
func Close() {
	mu.Lock()
	defer mu.Unlock()
	closeWritersLocked()
	commLogger.Store(nil)
	crashLogger.Store(nil)
	started = false
}

// closeWritersLocked 关闭全部滚动 writer（调用方必须持锁）
func closeWritersLocked() {
	for _, w := range writers {
		w.Close()
	}
	writers = nil
	appW = nil
}

// cleanOldLogs 删除超过保留天数的日志文件（按修改时间）
func cleanOldLogs(dir string, retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// TryEnsureDir 尝试创建目录并验证可写，成功返回 true
func TryEnsureDir(dir string) bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	// 验证目录可写
	tmp := filepath.Join(dir, ".write_test")
	if f, err := os.Create(tmp); err != nil {
		return false
	} else {
		f.Close()
		os.Remove(tmp)
		return true
	}
}
