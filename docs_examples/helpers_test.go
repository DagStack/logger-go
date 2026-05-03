// Test helpers shared across docs_examples_test files.
//
// The docs_examples package is an EXTERNAL test package (`docs_examples_test`)
// and cannot reach `package logger`'s internal `ResetRegistryForTests` helper.
// Tests therefore work around shared registry state by:
//
//  1. Configuring the root logger with a fresh InMemorySink at test start, and
//  2. Explicitly forcing every named logger they read from to ALSO point at
//     that same sink — this overrides whatever stale `sinksExplicit` flag a
//     prior test might have left on the cached logger.
//
// `installCapture` is the convenience helper that performs both steps.

package docs_examples_test

import (
	"testing"

	"go.dagstack.dev/logger"
)

// installCapture configures root with a fresh InMemorySink at TRACE level,
// then explicitly attaches the same sink to every named logger in `names`.
// Returns the capture sink. The caller must NOT call Configure(...) again
// inside the same test without re-running installCapture; that would reset
// root sinks while leaving the named loggers stale.
//
// names should include every logger.Get(name) the test uses for assertions.
// Pass the empty string "" to also pin the root logger explicitly.
func installCapture(t *testing.T, names ...string) *logger.InMemorySink {
	t.Helper()
	sink := logger.NewInMemorySink(1000, 1)
	logger.Configure(
		logger.WithRootLevel("TRACE"),
		logger.WithSinks(sink),
	)
	for _, n := range names {
		l := logger.Get(n)
		l.SetSinks([]logger.Sink{sink})
		// Force the named logger back to TRACE so an earlier test's
		// per-logger override (e.g., WithPerLoggerLevels{"<name>": "DEBUG"})
		// does not drop low-severity records during this test.
		l.SetMinSeverity(int(logger.SeverityTrace))
	}
	return sink
}
