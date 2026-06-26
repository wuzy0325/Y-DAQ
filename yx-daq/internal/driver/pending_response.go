package driver

import (
	"sync"
	"time"
)

// ResponseType defines how a command response is detected
type ResponseType int

const (
	ResponseNewline    ResponseType = iota // Response ends at \n
	ResponseFixedLength                    // Response has known fixed length
	ResponseSilenceWindow                 // Response ends after 30ms silence
)

// PendingEntry represents a pending command response expectation
type PendingEntry struct {
	Cmd         string
	RespType    ResponseType
	ExpectedLen int          // for ResponseFixedLength
	SilenceMs   int          // for ResponseSilenceWindow
	RespCh      chan string
	Deadline    time.Time
}

// PendingResponses FIFO queue of pending command response expectations
type PendingResponses struct {
	mu      sync.Mutex
	entries []*PendingEntry
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

// Len returns the number of pending entries
func (q *PendingResponses) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
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
