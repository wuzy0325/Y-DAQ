package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"

	"yx-daq/internal/app"
	"yx-daq/internal/logger"
	"yx-daq/internal/types"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 启动早期初始化日志（默认配置）：保证 application.New 之前的错误也能落盘；
	// 持久化配置在 Core.Startup 中通过 logger.Configure 应用。
	if err := logger.Init(types.DefaultLoggingConfig()); err != nil {
		println("logger init failed:", err.Error())
	}

	core := app.NewCore()

	// 前台权限移交：Windows 默认只允许前台进程调用 SetForegroundWindow。
	// 第二实例启动瞬间拥有前台权限，在此调用 AllowSetForegroundWindow(ASFW_ANY)
	// 把权限让渡出来，已运行实例才能将主窗口真正带到前台（权限不足时调用失败，无副作用）。
	allowAnyProcessSetForeground()

	// WebView2 数据目录：优先使用可执行文件所在目录下的 webview-data 子目录（兼容沙箱环境）
	webviewDataPath := ""
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "webview-data")
		if logger.TryEnsureDir(candidate) {
			webviewDataPath = candidate
		}
	}

	// 去掉 embed.FS 中的 "frontend/dist" 前缀，使请求 "/" 对应 "index.html"
	distFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		println("failed to create sub filesystem:", err.Error())
		return
	}

	a := application.New(application.Options{
		Name:        "YX-DAQ",
		Description: "YX-DAQ数据采集系统",
		Services: []application.Service{
			application.NewService(&app.CoreService{Core: core}),
			application.NewService(&app.DeviceService{Core: core}),
			application.NewService(&app.MotionService{Core: core}),
			application.NewService(&app.ThreeHoleService{Core: core}),
			application.NewService(&app.FiveHoleService{Core: core}),
			application.NewService(&app.CalibrationService{Core: core}),
			application.NewService(&app.DataService{Core: core}),
			application.NewService(&app.ConfigService{Core: core}),
			application.NewService(&app.LogService{Core: core}),
		},
		// 单实例：应用已在运行时，第二实例通知后立即退出（os.Exit），
		// 由已运行实例恢复并激活主窗口。
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.yx.yx-daq",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				slog.Info("检测到第二实例启动，已激活主窗口", "args", data.Args)
				if win, ok := application.Get().Window.Get("main"); ok && win != nil {
					win.Restore()
					win.Focus()
				}
			},
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(distFS),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			WebviewUserDataPath: webviewDataPath,
		},
	})

	mainWin := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "YX-DAQ数据采集系统",
		Width:            1440,
		Height:           900,
		MinWidth:         1280,
		MinHeight:        720,
		BackgroundColour: application.NewRGB(10, 10, 26),
		URL:              "/",
	})

	mainWin.Show()

	// 退出确认：用 hook 在 WindowClosing 事件派发到监听器之前同步拦截（hook 可取消事件）。
	// 首次点击关闭时取消本次关闭并弹出确认框；仅当用户确认后才放行默认关闭流程。
	// 确认后置位 exitConfirmed 并重新触发 Close()，使 hook 放行，随后执行下方监听器中的
	// core.Shutdown()（清理资源 + os.Exit）并关闭窗口。
	var exitConfirmed atomic.Bool
	mainWin.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if exitConfirmed.Load() {
			return // 用户已确认退出，放行关闭流程
		}
		// 取消本次关闭事件，保持窗口打开，等待用户在确认框中作出选择
		event.Cancel()

		dialog := a.Dialog.Question().
			SetTitle("退出确认").
			SetMessage("确定要退出 YX-DAQ 数据采集系统吗？")
		dialog.AttachToWindow(mainWin)
		// Windows MessageBox 仅支持标准按钮集，回调按 "Yes"/"No" 匹配，
		// 界面按钮由系统本地化（中文系统显示 是/否）
		dialog.AddButton("Yes").SetAsDefault().OnClick(func() {
			exitConfirmed.Store(true)
			mainWin.Close()
		})
		dialog.AddButton("No").SetAsCancel()
		dialog.Show()
	})

	// 主窗口关闭时直接清理并退出进程。
	// Wails v3 在 Windows 下关闭最后一个窗口后仅 PostQuitMessage，消息循环可能因
	// 后台协程或 InvokeSync 嵌套不退出，导致 a.Run() 不返回、ServiceShutdown 不触发，
	// 进而 Core.Shutdown() 中的 os.Exit 永远无法执行。在此显式触发清理 + 退出，
	// 不依赖消息循环退出流程，确保进程（含 wails3 dev 子进程）可靠终止。
	mainWin.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		core.Shutdown()
	})

	if err := a.Run(); err != nil {
		println("Error:", err.Error())
	}

	// 兜底：消息循环正常退出时确保进程终止（可能有后台协程阻止 Go runtime 自动退出）
	os.Exit(0)
}

// allowAnyProcessSetForeground 调用 user32!AllowSetForegroundWindow(ASFW_ANY)，
// 允许任意进程将窗口设为前台（用于第二实例把前台权限移交给已运行实例）。
func allowAnyProcessSetForeground() {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("AllowSetForegroundWindow")
	_, _, _ = proc.Call(uintptr(0xFFFFFFFF)) // ASFW_ANY
}
