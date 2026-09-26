package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// rotateWriter 按天 + 大小滚动的日志 writer。
// 按天：文件名内嵌日期，跨天自动切换到新文件（修复旧实现跨天仍写旧文件的问题）；
// 按大小：同日超出 maxSize 后创建带序号的滚动文件（prefix-YYYY-MM-DD.1.log）。
// 文件按需惰性打开：Close 后下次 Write 自动重建，ClearLogs 无需重建 handler。
type rotateWriter struct {
	mu      sync.Mutex
	dir     string
	prefix  string
	maxSize atomic.Int64 // 字节；<=0 表示不限制
	now     func() time.Time
	file    *os.File
	curDate string
	curSize int64
}

func newRotateWriter(dir, prefix string, maxSizeMB int) *rotateWriter {
	w := &rotateWriter{dir: dir, prefix: prefix, now: time.Now}
	w.SetMaxSizeMB(maxSizeMB)
	return w
}

// SetMaxSizeMB 更新单文件大小上限（运行时生效）
func (w *rotateWriter) SetMaxSizeMB(mb int) {
	if mb < 0 {
		mb = 0
	}
	w.maxSize.Store(int64(mb) * 1024 * 1024)
}

// Write 实现 io.Writer
func (w *rotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	date := w.now().Format(time.DateOnly)
	max := w.maxSize.Load()
	exceeded := max > 0 && w.curSize > 0 && w.curSize+int64(len(p)) > max
	switch {
	case w.file == nil || date != w.curDate:
		if err := w.openLocked(date, false); err != nil {
			return 0, err
		}
	case exceeded:
		if err := w.openLocked(date, true); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.curSize += int64(n)
	return n, err
}

// openLocked 打开目标文件（调用方必须持锁）。
// sizeRollover 为 true 时创建带序号的滚动文件，否则使用当天基础文件。
func (w *rotateWriter) openLocked(date string, sizeRollover bool) error {
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}

	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, date))
	if sizeRollover {
		max := w.maxSize.Load()
		for i := 1; ; i++ {
			candidate := filepath.Join(w.dir, fmt.Sprintf("%s-%s.%d.log", w.prefix, date, i))
			info, err := os.Stat(candidate)
			if err != nil || max <= 0 || info.Size() < max {
				path = candidate
				break
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}
	w.file = f
	w.curDate = date
	if info, err := f.Stat(); err == nil {
		w.curSize = info.Size()
	} else {
		w.curSize = 0
	}
	return nil
}

// Close 关闭当前文件（下次 Write 时自动重新打开）
func (w *rotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
