package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"yx-daq/internal/app"
	"yx-daq/internal/logger"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	core := app.NewCore()

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
