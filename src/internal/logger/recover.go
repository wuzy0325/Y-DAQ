package logger

import (
	"fmt"
	"log/slog"
	"runtime"
)

// Recover 捕获并记录 panic（用法：defer logger.Recover("receiveLoop")）。
// 完整堆栈写入 crash-YYYY-MM-DD.log，摘要写入主日志，避免后台 goroutine
// panic 导致进程直接退出且无任何痕迹。
func Recover(component string) {
	ReportPanic(component, recover())
}

// ReportPanic 记录已 recover 的 panic。
// 供需要自定义恢复逻辑（如 panic 后补发断连事件）的调用方使用。
func ReportPanic(component string, r any) {
	if r == nil {
		return
	}

	stack := make([]byte, 64<<10)
	stack = stack[:runtime.Stack(stack, false)]

	if l := crashLogger.Load(); l != nil {
		l.Error("panic",
			"component", component,
			"panic", fmt.Sprint(r),
			"stack", string(stack),
		)
	}
	slog.Error("panic recovered", "component", component, "panic", fmt.Sprint(r))
}
