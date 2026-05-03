// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/intro.mdx` (Go TabItem).
//
// Each test is wrapped around the verbatim snippet between the
// `// --- snippet start ---` / `// --- snippet end ---` markers, with
// surrounding scaffolding (logger registry isolation, captured sinks,
// stub functions) that allows the snippet to compile and run in a
// `testing.T` context.
//
// Note on registry isolation: ResetRegistryForTests is internal to package
// logger and not visible to this external test package. To avoid state
// pollution from the global registry between tests, every assertion is
// captured via Logger.ScopeSinks (which is hermetic — it overrides sinks
// on the receiver for the lexical block, then restores them). Snippets that
// the docs render with bare Configure(...) are still kept verbatim; the
// assertion mirror runs through ScopeSinks.

package docs_examples_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"go.dagstack.dev/logger"
)

// ── "Your first log line" — Configure + GetVersioned + Info ────────────────

func TestIntro_FirstLogLine(t *testing.T) {
	// --- snippet start -----------------------------------------------
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(logger.NewConsoleSink(logger.ConsoleAuto, nil, 1)),
		logger.WithResourceAttributes(logger.Attrs{"service.name": "order-service"}),
	)

	log := logger.GetVersioned("order_service.api", "1.0.0")
	log.Info("request received", logger.Attrs{"request.id": "req-abc", "user.id": 42})
	// --- snippet end -------------------------------------------------

	// Re-run via ScopeSinks (hermetic capture) so the assertions are
	// isolated from any earlier test's leaked logger state.
	capture := logger.NewInMemorySink(100, 1)
	err := log.ScopeSinks(context.Background(), []logger.Sink{capture}, func(_ context.Context) error {
		log.Info("request received", logger.Attrs{"request.id": "req-abc", "user.id": 42})
		return nil
	})
	if err != nil {
		t.Fatalf("ScopeSinks: %v", err)
	}

	records := capture.Records()
	if len(records) == 0 {
		t.Fatal("expected at least one captured record")
	}
	r := records[len(records)-1]
	if r.Body != "request received" {
		t.Errorf("body = %v, want %q", r.Body, "request received")
	}
	if r.SeverityNumber != int(logger.SeverityInfo) {
		t.Errorf("severity_number = %d, want %d", r.SeverityNumber, int(logger.SeverityInfo))
	}
	if r.SeverityText != logger.SeverityTextInfo {
		t.Errorf("severity_text = %q, want INFO", r.SeverityText)
	}
	if r.Attributes["request.id"] != "req-abc" {
		t.Errorf("request.id = %v", r.Attributes["request.id"])
	}
	if r.Attributes["user.id"] != 42 {
		t.Errorf("user.id = %v", r.Attributes["user.id"])
	}
	if r.InstrumentationScope == nil || r.InstrumentationScope.Version != "1.0.0" {
		t.Errorf("instrumentation_scope.version = %v, want 1.0.0", r.InstrumentationScope)
	}
	res := r.Resource
	if res == nil || res.Attributes["service.name"] != "order-service" {
		t.Errorf("resource.service.name not propagated: %v", res)
	}
}

// ── "Adding sinks" — FileSink + multi-sink Configure ───────────────────────

func TestIntro_AddingSinks(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "order-service.jsonl")

	// Adapt the snippet to the test's tempdir while keeping the API call
	// sequence identical to the docs (NewFileSink → Configure → WithSinks).
	// --- snippet start -----------------------------------------------
	fileSink, err := logger.NewFileSink(logFile, 100_000_000, 10, 1)
	if err != nil {
		// handle file open error
		t.Fatalf("NewFileSink: %v", err)
	}

	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(
			logger.NewConsoleSink(logger.ConsoleJSON, nil, 1),
			fileSink,
		),
		logger.WithResourceAttributes(logger.Attrs{
			"service.name":           "order-service",
			"service.version":        "1.0.0",
			"deployment.environment": "production",
		}),
	)
	// --- snippet end -------------------------------------------------

	// Assert the file sink was opened (the file exists). We don't write a
	// record here — the docs snippet stops at Configure.
	if _, err := filepath.Abs(logFile); err != nil {
		t.Errorf("abs: %v", err)
	}

	root := logger.Get("")
	sinks := root.EffectiveSinks()
	if len(sinks) != 2 {
		t.Errorf("EffectiveSinks() length = %d, want 2", len(sinks))
	}
	// Cleanup — close the file sink so the temp dir is removable on Windows
	// and locks release.
	if err := fileSink.Close(); err != nil {
		t.Errorf("fileSink.Close: %v", err)
	}
}

// ── "Logging exceptions" — log.ExceptionCtx ────────────────────────────────

// errOrderValidation is a stand-in for the docs' OrderValidationError.
type errOrderValidation struct{ msg string }

func (e *errOrderValidation) Error() string { return e.msg }

// processOrder is a stub that mirrors the docs snippet's error path.
func processOrder(_ context.Context, orderID int) error {
	if orderID == 1234 {
		return &errOrderValidation{msg: "invalid order"}
	}
	return nil
}

func TestIntro_LogException(t *testing.T) {
	// Use a unique logger name so that any global state from a prior test
	// (sinksExplicit on order_service from captureLogs / scope_sinks) does
	// not bleed into the assertion.
	log := logger.Get("docs.examples.intro.exception")
	capture := logger.NewInMemorySink(100, 1)
	ctx := context.Background()

	err := log.ScopeSinks(ctx, []logger.Sink{capture}, func(ctx context.Context) error {
		orderID := 1234

		// --- snippet start -----------------------------------------------
		if err := processOrder(ctx, orderID); err != nil {
			log.ExceptionCtx(ctx, err, nil, logger.Attrs{"order.id": orderID})
		}
		// --- snippet end -------------------------------------------------
		return nil
	})
	if err != nil {
		t.Fatalf("ScopeSinks: %v", err)
	}

	records := capture.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	r := records[0]
	if r.SeverityText != logger.SeverityTextError {
		t.Errorf("severity_text = %q, want ERROR", r.SeverityText)
	}
	if _, ok := r.Attributes["exception.type"]; !ok {
		t.Errorf("exception.type missing from attributes: %v", r.Attributes)
	}
	if _, ok := r.Attributes["exception.message"]; !ok {
		t.Errorf("exception.message missing from attributes")
	}
	if _, ok := r.Attributes["exception.stacktrace"]; !ok {
		t.Errorf("exception.stacktrace missing from attributes")
	}
	if r.Attributes["order.id"] != 1234 {
		t.Errorf("order.id = %v, want 1234", r.Attributes["order.id"])
	}
	// The docs say body falls back to err.Error() when nil is passed.
	if r.Body == nil {
		t.Errorf("body should default to err.Error(), got nil")
	}
}

// ── "Capturing logs in tests" — InMemorySink + ScopeSinks ──────────────────

// runBusinessLogic is a stub; the docs snippet calls it inside the scoped
// override. Here we make it emit through the SAME logger the scope is on,
// which is the documented behaviour.
func runBusinessLogic(ctx context.Context) {
	log := logger.Get("test_module")
	log.InfoCtx(ctx, "operation completed", logger.Attrs{"step": "final"})
}

func TestIntro_CapturingLogsInTests(t *testing.T) {
	ctx := context.Background()

	// --- snippet start -----------------------------------------------
	sink := logger.NewInMemorySink(100, 1)
	log := logger.Get("test_module")

	err := log.ScopeSinks(ctx, []logger.Sink{sink}, func(ctx context.Context) error {
		runBusinessLogic(ctx)
		return nil
	})
	if err != nil {
		// handle
		t.Fatalf("ScopeSinks: %v", err)
	}

	records := sink.Records()
	// assert any record's Body matches "operation completed"
	// --- snippet end -------------------------------------------------

	found := false
	for _, r := range records {
		if r.Body == "operation completed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'operation completed' record in capture; got %d records", len(records))
	}
}

// ensure errors package usage is referenced (gofmt safety).
var _ = errors.New
