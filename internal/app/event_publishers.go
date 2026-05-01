package app

import (
	"context"
	"yx-daq/internal/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// CalibrationEventPublisher 校准事件发布器实现
type CalibrationEventPublisher struct {
	ctx context.Context // Wails context
}

// EmitProgress 发送进度事件
func (p *CalibrationEventPublisher) EmitProgress(event types.CalibrationProgressEvent) {
	runtime.EventsEmit(p.ctx, "calibration:progress", event)
}

// EmitRealtime 发送实时数据事件
func (p *CalibrationEventPublisher) EmitRealtime(event types.CalibrationRealtimeEvent) {
	runtime.EventsEmit(p.ctx, "calibration:realtime", event)
}

// EmitComplete 发送完成事件
func (p *CalibrationEventPublisher) EmitComplete(event types.CalibrationCompleteEvent) {
	runtime.EventsEmit(p.ctx, "calibration:complete", event)
}

// ThreeHoleEventPublisher 三孔测试事件发布器实现
type ThreeHoleEventPublisher struct {
	ctx context.Context // Wails context
}

// EmitProgress 发送进度事件
func (p *ThreeHoleEventPublisher) EmitProgress(event types.ThreeHoleTraversalProgressEvent) {
	runtime.EventsEmit(p.ctx, "three-hole:progress", event)
}

// EmitRealtime 发送实时数据事件
func (p *ThreeHoleEventPublisher) EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent) {
	runtime.EventsEmit(p.ctx, "three-hole:realtime", event)
}

// EmitComplete 发送完成事件
func (p *ThreeHoleEventPublisher) EmitComplete(event types.ThreeHoleTraversalCompleteEvent) {
	runtime.EventsEmit(p.ctx, "three-hole:complete", event)
}

// EmitError 发送错误事件
func (p *ThreeHoleEventPublisher) EmitError(event types.ThreeHoleTraversalErrorEvent) {
	runtime.EventsEmit(p.ctx, "three-hole:error", event)
}