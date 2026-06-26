package app

import (
	"yx-daq/internal/types"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// CalibrationEventPublisher 校准事件发布器实现
type CalibrationEventPublisher struct {
	app *application.App
}

func (p *CalibrationEventPublisher) EmitProgress(event types.CalibrationProgressEvent) {
	p.app.Event.Emit("calibration:progress", event)
}

func (p *CalibrationEventPublisher) EmitRealtime(event types.CalibrationRealtimeEvent) {
	p.app.Event.Emit("calibration:realtime", event)
}

func (p *CalibrationEventPublisher) EmitComplete(event types.CalibrationCompleteEvent) {
	p.app.Event.Emit("calibration:complete", event)
}

// ThreeHoleEventPublisher 三孔测试事件发布器实现（支持多探针）
type ThreeHoleEventPublisher struct {
	app     *application.App
	probeID string
}

func NewThreeHoleEventPublisher(app *application.App, probeID string) *ThreeHoleEventPublisher {
	return &ThreeHoleEventPublisher{app: app, probeID: probeID}
}

func (p *ThreeHoleEventPublisher) channel(name string) string {
	return "three-hole:" + p.probeID + ":" + name
}

func (p *ThreeHoleEventPublisher) EmitProgress(event types.ThreeHoleTraversalProgressEvent) {
	p.app.Event.Emit(p.channel("progress"), event)
}

func (p *ThreeHoleEventPublisher) EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent) {
	p.app.Event.Emit(p.channel("realtime"), event)
}

func (p *ThreeHoleEventPublisher) EmitComplete(event types.ThreeHoleTraversalCompleteEvent) {
	p.app.Event.Emit(p.channel("complete"), event)
}

func (p *ThreeHoleEventPublisher) EmitError(event types.ThreeHoleTraversalErrorEvent) {
	p.app.Event.Emit(p.channel("error"), event)
}
