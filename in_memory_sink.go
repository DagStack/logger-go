package logger

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var inMemoryInstanceCounter atomic.Uint64

// InMemorySink accumulates LogRecords in a capacity-bounded ring buffer.
// Per spec ADR-0001 §7.2 — Phase 1 sink primarily used for tests and
// application self-checks.
//
// The oldest records are dropped automatically when capacity is exceeded.
type InMemorySink struct {
	capacity    int
	minSeverity int

	mu      sync.Mutex
	closed  bool
	records []*LogRecord
	id      string
}

// NewInMemorySink constructs a ring buffer with the given capacity. A
// capacity <= 0 is treated as 1 to keep the sink alive.
func NewInMemorySink(capacity int, minSeverity int) *InMemorySink {
	if capacity <= 0 {
		capacity = 1
	}
	// Per-instance suffix avoids ID collisions when several InMemorySinks
	// share the same capacity (common in tests).
	return &InMemorySink{
		capacity:    capacity,
		minSeverity: minSeverity,
		records:     make([]*LogRecord, 0, capacity),
		id:          fmt.Sprintf("in-memory:cap=%d#%d", capacity, inMemoryInstanceCounter.Add(1)),
	}
}

// ID returns the URI-style sink identifier ("in-memory:cap=<N>#<seq>").
func (s *InMemorySink) ID() string { return s.id }

// SupportsSeverity reports whether severityNumber meets the minimum.
func (s *InMemorySink) SupportsSeverity(severityNumber int) bool {
	return severityNumber >= s.minSeverity
}

// Capacity returns the configured ring buffer size.
func (s *InMemorySink) Capacity() int { return s.capacity }

// Emit appends record to the ring; if at capacity, drops the oldest entry.
func (s *InMemorySink) Emit(record *LogRecord) {
	if record == nil {
		return
	}
	if !s.SupportsSeverity(record.SeverityNumber) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if len(s.records) >= s.capacity {
		copy(s.records, s.records[1:])
		s.records = s.records[:s.capacity-1]
	}
	s.records = append(s.records, record)
}

// Flush is a no-op for in-memory storage.
func (s *InMemorySink) Flush(_ float64) error { return nil }

// Close marks the sink as closed; subsequent Emit calls become no-ops.
// Captured records remain accessible via Records.
func (s *InMemorySink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// Records returns a snapshot copy of the captured records. Mutating the
// returned slice does not affect the sink's internal buffer.
func (s *InMemorySink) Records() []*LogRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*LogRecord, len(s.records))
	copy(out, s.records)
	return out
}

// Clear empties the captured-records buffer. The sink remains usable.
func (s *InMemorySink) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = s.records[:0]
}
