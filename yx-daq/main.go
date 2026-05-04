package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"yx-daq/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

type App struct {
	*app.App
}

func NewApp() *App {
	return &App{App: app.NewApp()}
}

func main() {
	appInstance := NewApp()

	err := wails.Run(&options.App{
		Title:     "YX-DAQ数据采集系统",
		Width:     1440,
		Height:    900,
		MinWidth:  1280,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 26, A: 255},
		OnStartup: func(ctx context.Context) {
			appInstance.Startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			appInstance.Shutdown(ctx)
		},
		Bind: []any{
			appInstance,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
