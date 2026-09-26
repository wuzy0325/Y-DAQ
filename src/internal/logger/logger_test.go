package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yx-daq/internal/types"
)

func readFileOrEmpty(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		" warn ":  slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"":        slog.LevelInfo,
		"bogus":   slog.LevelInfo,
	}
	for input, want := range cases {
		if got := ParseLevel(input); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestValidLevel(t *testing.T) {
	for _, ok := range []string{"debug", "info", "warn", "error", "WARNING"} {
		if !ValidLevel(ok) {
			t.Errorf("ValidLevel(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "trace", "verbose"} {
		if ValidLevel(bad) {
			t.Errorf("ValidLevel(%q) = true, want false", bad)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("hello", 10); got != "hello" {
		t.Errorf("Truncate short = %q", got)
	}
	if got := Truncate("hello世界", 7); got != "hello世界" {
		t.Errorf("Truncate exact = %q", got)
	}
	if got := Truncate("hello世界", 6); !strings.HasPrefix(got, "hello世") || !strings.Contains(got, "truncated") {
		t.Errorf("Truncate long = %q", got)
	}
}

func TestRotateWriterDayRollover(t *testing.T) {
	dir := t.TempDir()
	w := newRotateWriter(dir, "app", 0)
	cur := time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC)
	w.now = func() time.Time { return cur }

	if _, err := w.Write([]byte("day1\n")); err != nil {
		t.Fatalf("write day1: %v", err)
	}
	cur = cur.Add(2 * time.Hour)
	if _, err := w.Write([]byte("day2\n")); err != nil {
		t.Fatalf("write day2: %v", err)
	}
	w.Close()

	if got := readFileOrEmpty(t, filepath.Join(dir, "app-2026-01-01.log")); !strings.Contains(got, "day1") {
		t.Errorf("day1 file = %q", got)
	}
	if got := readFileOrEmpty(t, filepath.Join(dir, "app-2026-01-02.log")); !strings.Contains(got, "day2") {
		t.Errorf("day2 file = %q", got)
	}
}

func TestRotateWriterSizeRollover(t *testing.T) {
	dir := t.TempDir()
	w := newRotateWriter(dir, "app", 0)
	cur := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)
	w.now = func() time.Time { return cur }
	w.maxSize.Store(20)

	if _, err := w.Write([]byte("1234567890\n")); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if _, err := w.Write([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	w.Close()

	base := readFileOrEmpty(t, filepath.Join(dir, "app-2026-02-03.log"))
	rolled := readFileOrEmpty(t, filepath.Join(dir, "app-2026-02-03.1.log"))
	if !strings.Contains(base, "1234567890") || strings.Contains(base, "abcdefghij") {
		t.Errorf("base file = %q", base)
	}
	if !strings.Contains(rolled, "abcdefghij") {
		t.Errorf("rolled file = %q", rolled)
	}
}

func TestLevelFiltering(t *testing.T) {
	dir := t.TempDir()
	cfg := types.DefaultLoggingConfig()
	cfg.Level = "warn"
	cfg.Console = false
	if err := InitAt(dir, cfg); err != nil {
		t.Fatalf("init: %v", err)
	}
	slog.Info("hidden-info")
	slog.Warn("visible-warn")
	Close()

	got := readFileOrEmpty(t, filepath.Join(dir, "yx-daq-"+time.Now().Format("2006-01-02")+".log"))
	if strings.Contains(got, "hidden-info") {
		t.Errorf("info log should be filtered, got %q", got)
	}
	if !strings.Contains(got, "visible-warn") {
		t.Errorf("warn log missing, got %q", got)
	}
}

func TestCommErrorWritesCommFileOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := types.DefaultLoggingConfig()
	cfg.Console = false
	if err := InitAt(dir, cfg); err != nil {
		t.Fatalf("init: %v", err)
	}
	CommError("TCP write failed", "device", "dev-1", "err", "boom")
	Close()

	comm := readFileOrEmpty(t, filepath.Join(dir, "comm-"+time.Now().Format("2006-01-02")+".log"))
	if !strings.Contains(comm, "TCP write failed") || !strings.Contains(comm, "dev-1") {
		t.Errorf("comm file = %q", comm)
	}
	app := readFileOrEmpty(t, filepath.Join(dir, "yx-daq-"+time.Now().Format("2006-01-02")+".log"))
	if strings.Contains(app, "TCP write failed") {
		t.Errorf("comm error should not leak into app log, got %q", app)
	}
}

func TestCommErrorDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := types.DefaultLoggingConfig()
	cfg.Console = false
	cfg.CommEnabled = false
	if err := InitAt(dir, cfg); err != nil {
		t.Fatalf("init: %v", err)
	}
	CommError("should-not-appear", "err", "boom")
	Close()

	comm := readFileOrEmpty(t, filepath.Join(dir, "comm-"+time.Now().Format("2006-01-02")+".log"))
	if strings.Contains(comm, "should-not-appear") {
		t.Errorf("comm logging disabled but got %q", comm)
	}
}

func TestRecoverWritesCrashFile(t *testing.T) {
	dir := t.TempDir()
	cfg := types.DefaultLoggingConfig()
	cfg.Console = false
	if err := InitAt(dir, cfg); err != nil {
		t.Fatalf("init: %v", err)
	}

	func() {
		defer Recover("unit-test-component")
		panic("boom-panic")
	}()
	Close()

	crash := readFileOrEmpty(t, filepath.Join(dir, "crash-"+time.Now().Format("2006-01-02")+".log"))
	if !strings.Contains(crash, "unit-test-component") || !strings.Contains(crash, "boom-panic") {
		t.Errorf("crash file = %q", crash)
	}
}

func TestConfigureRuntime(t *testing.T) {
	dir := t.TempDir()
	cfg := types.DefaultLoggingConfig()
	cfg.Console = false
	if err := InitAt(dir, cfg); err != nil {
		t.Fatalf("init: %v", err)
	}

	next := cfg
	next.Level = "error"
	next.CommEnabled = false
	Configure(next)
	if got := GetConfig(); got.Level != "error" || got.CommEnabled {
		t.Errorf("GetConfig after Configure = %+v", got)
	}
	Close()
}
