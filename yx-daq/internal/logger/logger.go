package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
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

func Init() error {
	mu.Lock()
	defer mu.Unlock()

	// 优先使用可执行文件所在目录下的 logs 子目录（兼容沙箱环境）
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "logs")
		if TryEnsureDir(candidate) {
			logDir = candidate
		}
	}

	// 回退到系统标准目录
	if logDir == "" {
		if runtime.GOOS == "windows" {
			localAppData := os.Getenv("LOCALAPPDATA")
			if localAppData == "" {
				localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
			}
			logDir = filepath.Join(localAppData, appName, "logs")
		} else {
			home, _ := os.UserHomeDir()
			logDir = filepath.Join(home, ".local", "share", appName, "logs")
		}
	}

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
