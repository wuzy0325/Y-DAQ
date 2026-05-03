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

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		logDir = filepath.Join(localAppData, appName, "logs")
	} else {
		home, _ := os.UserHomeDir() // 无HOME则用空字符串（后续路径拼接降级为相对路径）
		logDir = filepath.Join(home, ".local", "share", appName, "logs")
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

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if closer != nil {
		closer.Close()
		closer = nil
	}
}
