package logger

// Sink is the destination for LogRecords per spec ADR-0001 §7.1.
//
// Implementations must be safe for concurrent use. Phase 1 sinks
// (ConsoleSink, FileSink, InMemorySink) use synchronous local I/O — the
// non-blocking property of Logger.Info is provided by OS buffering. Phase 2
// sinks (OTLPSink, ...) will introduce true async batching with internal
// queues; Emit then enqueues the record for a worker.
//
// ID is a URI-style identifier used in diagnostics and metrics:
//
//	"console:json"
//	"file:/var/log/app.jsonl"
//	"in-memory:cap=100#1"
type Sink interface {
	// ID returns a URI-style identifier for diagnostics.
	ID() string

	// Emit delivers a record to the sink. Phase 1: synchronous; Phase 2: enqueue.
	// Errors during emit are absorbed by the sink and surfaced via metrics or
	// the dagstack.logger.internal diagnostic channel — Emit itself never
	// blocks the caller and never returns an error.
	Emit(record *LogRecord)

	// Flush blocks until buffered records are delivered, or until the
	// timeout is exhausted. Phase 1 sinks are synchronous, so
	// timeoutSeconds is accepted for forward compatibility but is NOT
	// enforced — Phase 1 implementations never return a timeout error.
	// Phase 2 (OTLPSink and friends) MUST honour the deadline and
	// return a wrapped context.DeadlineExceeded.
	Flush(timeoutSeconds float64) error

	// Close flushes pending records and releases resources. Idempotent.
	Close() error

	// SupportsSeverity is the early-drop hint: returns false when the sink
	// will not emit a record at the given severity_number. Logger uses this
	// to avoid building a record for sinks that would discard it.
	SupportsSeverity(severityNumber int) bool
}
