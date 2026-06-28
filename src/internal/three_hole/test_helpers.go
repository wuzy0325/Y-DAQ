package three_hole

import (
	"yx-daq/internal/types"
)

// MockEventPublisher 用于测试的模拟事件发布器
type MockEventPublisher struct {
	progressEvents []types.ThreeHoleTraversalProgressEvent
	completeEvents []types.ThreeHoleTraversalCompleteEvent
	errorEvents    []types.ThreeHoleTraversalErrorEvent
	realtimeEvents []types.ThreeHoleTraversalRealtimeEvent
}

func (m *MockEventPublisher) EmitProgress(event types.ThreeHoleTraversalProgressEvent) {
	m.progressEvents = append(m.progressEvents, event)
}

func (m *MockEventPublisher) EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent) {
	m.realtimeEvents = append(m.realtimeEvents, event)
}

func (m *MockEventPublisher) EmitComplete(event types.ThreeHoleTraversalCompleteEvent) {
	m.completeEvents = append(m.completeEvents, event)
}

func (m *MockEventPublisher) EmitError(event types.ThreeHoleTraversalErrorEvent) {
	m.errorEvents = append(m.errorEvents, event)
}

func (m *MockEventPublisher) GetProgressEvents() []types.ThreeHoleTraversalProgressEvent {
	return m.progressEvents
}

func (m *MockEventPublisher) GetCompleteEvents() []types.ThreeHoleTraversalCompleteEvent {
	return m.completeEvents
}

func (m *MockEventPublisher) GetErrorEvents() []types.ThreeHoleTraversalErrorEvent {
	return m.errorEvents
}

func (m *MockEventPublisher) GetRealtimeEvents() []types.ThreeHoleTraversalRealtimeEvent {
	return m.realtimeEvents
}

func (m *MockEventPublisher) Clear() {
	m.progressEvents = nil
	m.completeEvents = nil
	m.errorEvents = nil
	m.realtimeEvents = nil
}