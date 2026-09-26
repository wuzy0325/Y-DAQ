package driver

import (
	"sync"
	"time"
)

// ResponseType defines how a command response is detected
type ResponseType int

const (
	ResponseNewline       ResponseType = iota // Response ends at \n
	ResponseFixedLength                       // Response has known fixed length
	ResponseSilenceWindow                     // Response ends after 30ms silence
)

// PendingEntry represents a pending command response expectation
type PendingEntry struct {
	Cmd         string
	RespType    ResponseType
	ExpectedLen int // for ResponseFixedLength
	SilenceMs   int // for ResponseSilenceWindow
	RespCh      chan string
	Deadline    time.Time
}

// PendingResponses FIFO queue of pending command response expectations
type PendingResponses struct {
	mu      sync.Mutex
	entries []*PendingEntry
}

// pendingDispatch 待投递的响应结果（entry + 响应文本）。
// handleCommandResponse 在持锁阶段只收集，统一在锁外投递，避免持有 mu 时阻塞 channel 发送
type pendingDispatch struct {
	entry *PendingEntry
	resp  string
}

// dispatchPending 在锁外统一投递响应结果到各 entry 的 RespCh。
// RespCh 为容量 1 的缓冲 channel，正常每个 entry 只投递一次，不会阻塞
func dispatchPending(sends []pendingDispatch) {
	for _, s := range sends {
		s.entry.RespCh <- s.resp
	}
}

// NewPendingResponses creates a new empty pending responses queue
func NewPendingResponses() *PendingResponses {
	return &PendingResponses{}
}

// Push adds a new pending entry to the back of the queue
func (q *PendingResponses) Push(entry *PendingEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries = append(q.entries, entry)
}

// Pop removes and returns the front entry, or nil if empty
func (q *PendingResponses) Pop() *PendingEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) == 0 {
		return nil
	}
	entry := q.entries[0]
	q.entries = q.entries[1:]
	return entry
}

// Front returns the front entry without removing it, or nil if empty
func (q *PendingResponses) Front() *PendingEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) == 0 {
		return nil
	}
	return q.entries[0]
}

// IsEmpty returns whether the queue is empty
func (q *PendingResponses) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries) == 0
}

// RemoveByCmd removes the first entry matching the given command and returns it, or nil
func (q *PendingResponses) RemoveByCmd(cmd string) *PendingEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, e := range q.entries {
		if e.Cmd == cmd {
			q.entries = append(q.entries[:i], q.entries[i+1:]...)
			return e
		}
	}
	return nil
}

// Clear 清空队列并返回所有被移除的 entry（调用方负责唤醒其 RespCh）。
// 用于重连场景：旧连接未完成的响应期望在新连接上不会被兑现，
// 残留 entry 会被新连接的首个响应误匹配（队列错位）。
func (q *PendingResponses) Clear() []*PendingEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	entries := q.entries
	q.entries = nil
	return entries
}

// RemoveExpired removes and returns all expired entries
func (q *PendingResponses) RemoveExpired() []*PendingEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	var expired []*PendingEntry
	now := time.Now()
	i := 0
	for i < len(q.entries) {
		if now.After(q.entries[i].Deadline) {
			expired = append(expired, q.entries[i])
			q.entries = append(q.entries[:i], q.entries[i+1:]...)
		} else {
			i++
		}
	}
	return expired
}
