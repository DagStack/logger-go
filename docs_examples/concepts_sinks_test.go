// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/concepts/sinks.mdx` (Go TabItem).

package docs_examples_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dagstack.dev/logger"
)

// ── "ConsoleSink" — three construction modes ──────────────────────────────

func TestSinks_ConsoleConstructors(t *testing.T) {
	// --- snippet start -----------------------------------------------
	// import (
	//     "os"
	//     "go.dagstack.dev/logger"
	// )

	// Auto mode: pretty on a TTY, JSON otherwise. Pass nil for stream → os.Stderr.
	sink := logger.NewConsoleSink(logger.ConsoleAuto, nil, 1)

	// Force JSON for container logs.
	sink = logger.NewConsoleSink(logger.ConsoleJSON, os.Stdout, int(logger.SeverityInfo))

	// Force pretty for a debug terminal.
	sink = logger.NewConsoleSink(logger.ConsolePretty, nil, 1)
	// --- snippet end -------------------------------------------------

	// All three constructors must produce a non-nil sink with a recognisable
	// ID prefix.
	if sink == nil {
		t.Fatal("NewConsoleSink returned nil")
	}
	if got := sink.ID(); got != "console:pretty" {
		t.Errorf("ID() = %q, want console:pretty", got)
	}

	// Independently verify the auto and json constructors too.
	auto := logger.NewConsoleSink(logger.ConsoleAuto, nil, 1)
	if got := auto.ID(); got != "console:auto" {
		t.Errorf("auto.ID() = %q, want console:auto", got)
	}
	jsonSink := logger.NewConsoleSink(logger.ConsoleJSON, os.Stdout, int(logger.SeverityInfo))
	if got := jsonSink.ID(); got != "console:json" {
		t.Errorf("json.ID() = %q, want console:json", got)
	}
	if !jsonSink.SupportsSeverity(int(logger.SeverityInfo)) {
		t.Errorf("SupportsSeverity(INFO) = false, want true")
	}
	if jsonSink.SupportsSeverity(int(logger.SeverityDebug)) {
		t.Errorf("SupportsSeverity(DEBUG) = true, want false (min=INFO)")
	}
}

// ── "FileSink" — error-returning constructor ──────────────────────────────

func TestSinks_FileSink(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "order-service.jsonl")

	// --- snippet start -----------------------------------------------
	sink, err := logger.NewFileSink(
		logPath,
		100_000_000,              // maxBytes — rotate at 100 MB
		10,                       // keep — keep 10 archived files
		int(logger.SeverityInfo), // minSeverity — INFO and above
	)
	if err != nil {
		// failed to open file
		t.Fatalf("NewFileSink: %v", err)
	}
	// --- snippet end -------------------------------------------------

	defer sink.Close()
	if sink == nil {
		t.Fatal("NewFileSink returned nil sink")
	}
	if !sink.SupportsSeverity(int(logger.SeverityInfo)) {
		t.Error("SupportsSeverity(INFO) = false, want true")
	}
	if sink.SupportsSeverity(int(logger.SeverityDebug)) {
		t.Error("SupportsSeverity(DEBUG) = true, want false (min=INFO)")
	}
	// File must exist after construction.
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

// ── "InMemorySink" — Records / Clear ──────────────────────────────────────

func TestSinks_InMemorySink(t *testing.T) {
	// --- snippet start -----------------------------------------------
	sink := logger.NewInMemorySink(100, 1) // capacity=100, minSeverity=1
	// ... emit some records ...

	records := sink.Records() // snapshot copy
	// assertions on records[i].Body, records[i].Attributes, ...

	sink.Clear() // reset for the next test
	// --- snippet end -------------------------------------------------

	if got := len(records); got != 0 {
		t.Errorf("Records() before any emit = %d, want 0", got)
	}

	// Emit a couple of records and re-check. Use installCapture to ensure
	// the named logger is wired explicitly to `sink`, irrespective of any
	// stale state from earlier tests.
	logger.Configure(logger.WithRootLevel("TRACE"), logger.WithSinks(sink))
	log := logger.Get("docs.examples.sinks.inmemory")
	log.SetSinks([]logger.Sink{sink})
	log.Info("first", nil)
	log.Info("second", nil)

	got := sink.Records()
	if len(got) != 2 {
		t.Errorf("Records() length after 2 emits = %d, want 2", len(got))
	}

	// Snapshot copy semantics — clearing the sink must not mutate the
	// already-returned slice (the docs comment "snapshot copy").
	snapshot := sink.Records()
	sink.Clear()
	if len(snapshot) != 2 {
		t.Errorf("snapshot mutated after Clear: %d, want 2", len(snapshot))
	}
	if got2 := sink.Records(); len(got2) != 0 {
		t.Errorf("Records() after Clear = %d, want 0", len(got2))
	}
}

// ── "Multi-sink routing" — per-sink min_severity ──────────────────────────

func TestSinks_MultiSinkRouting(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.jsonl")

	// --- snippet start -----------------------------------------------
	fileSink, _ := logger.NewFileSink(logPath, 100_000_000, 10, int(logger.SeverityInfo))

	logger.Configure(
		logger.WithRootLevel("DEBUG"),
		logger.WithSinks(
			logger.NewConsoleSink(logger.ConsolePretty, nil, int(logger.SeverityWarn)), // WARN+ on the console
			fileSink,
		),
	)
	// --- snippet end -------------------------------------------------

	defer fileSink.Close()

	// We can't directly observe ConsoleSink output without redirecting
	// stderr, but we CAN assert that the EffectiveSinks list has length 2
	// and that each sink's SupportsSeverity reflects the configured floor.
	root := logger.Get("")
	sinks := root.EffectiveSinks()
	if len(sinks) != 2 {
		t.Fatalf("EffectiveSinks length = %d, want 2", len(sinks))
	}

	if !sinks[0].SupportsSeverity(int(logger.SeverityWarn)) {
		t.Error("console sink should support WARN")
	}
	if sinks[0].SupportsSeverity(int(logger.SeverityInfo)) {
		t.Error("console sink should NOT support INFO (floor=WARN)")
	}
	if !sinks[1].SupportsSeverity(int(logger.SeverityInfo)) {
		t.Error("file sink should support INFO")
	}
	if sinks[1].SupportsSeverity(int(logger.SeverityDebug)) {
		t.Error("file sink should NOT support DEBUG (floor=INFO)")
	}

	// Cleanup so subsequent tests start with a fresh root.
	root.SetSinks(nil)
}
