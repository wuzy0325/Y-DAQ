package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	appName    = "yx-daq"
	maxLogDays = 30
)

var (
	mu     sync.Mutex
	logDir string
	closer io.Closer
)

// Init 初始化日志系统。
// 日志固定写入 ~/.yx-daq/logs/yx-daq-YYYY-MM-DD.log，保留 30 天。
// 使用用户主目录避免 Program Files 等 UAC 虚拟化目录导致写入失效
// （非管理员运行时对 Program Files 的写入会被透明重定向到 VirtualStore，
// 导致目标路径下的文件成为 0 字节空壳）。
func Init() error {
	mu.Lock()
	defer mu.Unlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get user home dir: %w", err)
	}
	logDir = filepath.Join(home, ".yx-daq", "logs")

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	cleanOldLogs()

	file, err := os.OpenFile(todayLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	closer = file

	multiWriter := io.MultiWriter(os.Stderr, file)

	handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))

	slog.Info("logger initialized", "dir", logDir)
	return nil
}

func todayLogPath() string {
	return filepath.Join(logDir, fmt.Sprintf("%s-%s.log", appName, time.Now().Format("2006-01-02")))
}

func cleanOldLogs() {
	cutoff := time.Now().AddDate(0, 0, -maxLogDays)
	entries, err := os.ReadDir(logDir)
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
			os.Remove(filepath.Join(logDir, e.Name()))
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

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if closer != nil {
		closer.Close()
		closer = nil
	}
}
