package main

import (
	"embed"

	"yx-daq/internal/app"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	core := app.NewCore()

	a := application.New(application.Options{
		Name:        "YX-DAQ",
		Description: "YX-DAQ数据采集系统",
		Services: []application.Service{
			application.NewService(&app.CoreService{Core: core}),
			application.NewService(&app.DeviceService{Core: core}),
			application.NewService(&app.MotionService{Core: core}),
			application.NewService(&app.ThreeHoleService{Core: core}),
			application.NewService(&app.CalibrationService{Core: core}),
			application.NewService(&app.DataService{Core: core}),
			application.NewService(&app.ConfigService{Core: core}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	a.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "YX-DAQ数据采集系统",
		Width:            1440,
		Height:           900,
		MinWidth:         1280,
		MinHeight:        720,
		BackgroundColour: application.NewRGB(10, 10, 26),
		URL:              "/",
	})

	if err := a.Run(); err != nil {
		println("Error:", err.Error())
	}
}
