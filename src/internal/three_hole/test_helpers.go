package three_hole

import (
	"sync"

	"yx-daq/internal/types"
)

// MockEventPublisher 用于测试的模拟事件发布器
// 含 sync.Mutex 保护，可安全用于 realtime monitor 等并发场景
type MockEventPublisher struct {
	mu             sync.Mutex
	progressEvents []types.ThreeHoleTraversalProgressEvent
	completeEvents []types.ThreeHoleTraversalCompleteEvent
	errorEvents    []types.ThreeHoleTraversalErrorEvent
	realtimeEvents []types.ThreeHoleTraversalRealtimeEvent
}

func (m *MockEventPublisher) EmitProgress(event types.ThreeHoleTraversalProgressEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressEvents = append(m.progressEvents, event)
}

func (m *MockEventPublisher) EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.realtimeEvents = append(m.realtimeEvents, event)
}

func (m *MockEventPublisher) EmitComplete(event types.ThreeHoleTraversalCompleteEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeEvents = append(m.completeEvents, event)
}

func (m *MockEventPublisher) EmitError(event types.ThreeHoleTraversalErrorEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorEvents = append(m.errorEvents, event)
}

func (m *MockEventPublisher) GetProgressEvents() []types.ThreeHoleTraversalProgressEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.progressEvents
}

func (m *MockEventPublisher) GetCompleteEvents() []types.ThreeHoleTraversalCompleteEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.completeEvents
}

func (m *MockEventPublisher) GetErrorEvents() []types.ThreeHoleTraversalErrorEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errorEvents
}

func (m *MockEventPublisher) GetRealtimeEvents() []types.ThreeHoleTraversalRealtimeEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.realtimeEvents
}

func (m *MockEventPublisher) RealtimeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.realtimeEvents)
}

func (m *MockEventPublisher) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressEvents = nil
	m.completeEvents = nil
	m.errorEvents = nil
	m.realtimeEvents = nil
}
