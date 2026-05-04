// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/guides/custom-sink.mdx` (Go TabItem).
//
// The page demonstrates how a downstream user implements the four-method
// Sink interface. We mirror the CallbackSink struct and constructor verbatim
// (the docs example), then exercise it through a real Configure call.

package docs_examples_test

import (
	"fmt"
	"sync"
	"testing"

	"go.dagstack.dev/logger"
)

// --- snippet start (Step 2 — CallbackSink type + receivers) ---------------

// CallbackSink forwards each LogRecord to a user-supplied callback.
type CallbackSink struct {
	callback    func(*logger.LogRecord)
	minSeverity int
	id          string

	mu     sync.Mutex
	closed bool
}

func NewCallbackSink(callback func(*logger.LogRecord), minSeverity int) *CallbackSink {
	return &CallbackSink{
		callback:    callback,
		minSeverity: minSeverity,
		id:          fmt.Sprintf("callback:%p", callback),
	}
}

func (s *CallbackSink) ID() string { return s.id }

func (s *CallbackSink) SupportsSeverity(severityNumber int) bool {
	return severityNumber >= s.minSeverity
}

func (s *CallbackSink) Emit(record *logger.LogRecord) {
	if record == nil {
		return
	}
	if !s.SupportsSeverity(record.SeverityNumber) {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.callback(record)
}

func (s *CallbackSink) Flush(_ float64) error { return nil }

func (s *CallbackSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// --- snippet end (Step 2) -------------------------------------------------

// ── Verify the type satisfies logger.Sink at compile time ─────────────────
var _ logger.Sink = (*CallbackSink)(nil)

func TestCustomSink_BasicEmit(t *testing.T) {
	var got []*logger.LogRecord
	cb := func(r *logger.LogRecord) { got = append(got, r) }

	sink := NewCallbackSink(cb, int(logger.SeverityInfo))
	if !sink.SupportsSeverity(int(logger.SeverityInfo)) {
		t.Error("SupportsSeverity(INFO) = false, want true")
	}
	if sink.SupportsSeverity(int(logger.SeverityDebug)) {
		t.Error("SupportsSeverity(DEBUG) = true, want false (floor=INFO)")
	}

	logger.Configure(
		logger.WithRootLevel("DEBUG"),
		logger.WithSinks(sink),
	)
	log := logger.Get("docs.examples.custom_sink.basic")
	log.Debug("filtered", nil) // below floor
	log.Info("captured", nil)
	log.Error("captured too", nil)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (DEBUG should be filtered by sink)", len(got))
	}
	if got[0].Body != "captured" {
		t.Errorf("got[0].Body = %v, want 'captured'", got[0].Body)
	}
	if got[1].SeverityText != logger.SeverityTextError {
		t.Errorf("got[1].SeverityText = %q, want ERROR", got[1].SeverityText)
	}

	// Idempotent close (the docs' "Step 6" testing checklist).
	if err := sink.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}

	// After close, Emit must be a no-op.
	prevLen := len(got)
	log.Info("after close", nil)
	if len(got) != prevLen {
		t.Errorf("Emit ran after Close: len went %d → %d", prevLen, len(got))
	}
}

// ── Wire-up snippet — alongside the built-in sinks ────────────────────────

// forwardToSentry stands in for the docs' Sentry forwarder. The docs version
// imports github.com/getsentry/sentry-go which is not a dependency of this
// repo; we keep the function shape and severity gate identical and substitute
// a slice append for the SDK call. The substitution is documented in the
// caller comment and clearly outside the verbatim snippet markers below.
var sentryEvents []string

func forwardToSentry(record *logger.LogRecord) {
	// --- snippet start (forwardToSentry shape) ----------------------------
	if record.SeverityNumber >= int(logger.SeverityError) {
		// Substitution: docs use sentry.NewEvent / CaptureEvent here.
		// The shape (severity gate + Body access) is identical.
		sentryEvents = append(sentryEvents, fmt.Sprint(record.Body))
	}
	// --- snippet end (forwardToSentry shape) ------------------------------
}

func TestCustomSink_WireUp(t *testing.T) {
	sentryEvents = nil

	// --- snippet start (Configure with built-in + custom sink) ------------
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(
			logger.NewConsoleSink(logger.ConsoleAuto, nil, 1),
			NewCallbackSink(forwardToSentry, int(logger.SeverityError)),
		),
	)
	// --- snippet end ------------------------------------------------------

	log := logger.Get("docs.examples.custom_sink.wireup")
	log.Info("not forwarded", nil)
	log.Error("forwarded", nil)
	log.Fatal("forwarded too", nil)

	if len(sentryEvents) != 2 {
		t.Errorf("len(sentryEvents) = %d, want 2 (only ERROR+ forwarded)", len(sentryEvents))
	}
	if sentryEvents[0] != "forwarded" {
		t.Errorf("sentryEvents[0] = %q, want 'forwarded'", sentryEvents[0])
	}

	// Reset so subsequent tests start clean.
	logger.Get("").SetSinks(nil)
}
