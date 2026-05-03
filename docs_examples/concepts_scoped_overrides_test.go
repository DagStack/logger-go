// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/concepts/scoped-overrides.mdx` (Go TabItem).

package docs_examples_test

import (
	"context"
	"path/filepath"
	"testing"

	"go.dagstack.dev/logger"
)

// ── "Three operations" — WithSinks, AppendSinks, WithoutSinks ─────────────

func TestScopedOverrides_ThreeOperations(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.jsonl")

	// Pre-attach a baseline sink to the parent so AppendSinks has something
	// to append to. Without this the docs claim "the parent's plus the extras"
	// has no parent sinks.
	parentCapture := installCapture(t, "order_service")

	// --- snippet start -----------------------------------------------
	log := logger.Get("order_service")

	// Replace sinks — only InMemorySink receives emits.
	testLog := log.WithSinks(logger.NewInMemorySink(100, 1))
	testLog.Info("captured here", nil)

	// Append a sink — both the parent's and the extra receive emits.
	fileSink, _ := logger.NewFileSink(auditPath, 0, 0, 1)
	auditLog := log.AppendSinks(fileSink)
	auditLog.Info("audit event", nil)

	// Discard — emits go to /dev/null.
	silentLog := log.WithoutSinks()
	silentLog.Info("never seen", nil)
	// --- snippet end -------------------------------------------------

	defer fileSink.Close()

	// "captured here" — went to the WithSinks override, NOT to parentCapture.
	for _, r := range parentCapture.Records() {
		if r.Body == "captured here" {
			t.Errorf("WithSinks did not isolate: parentCapture saw %q", r.Body)
		}
	}
	// "audit event" — should appear in parentCapture (because AppendSinks
	// preserves the parent's sinks).
	foundAudit := false
	for _, r := range parentCapture.Records() {
		if r.Body == "audit event" {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Errorf("AppendSinks lost parent sinks: 'audit event' not in parentCapture")
	}
	// "never seen" — must NOT appear anywhere.
	for _, r := range parentCapture.Records() {
		if r.Body == "never seen" {
			t.Errorf("WithoutSinks emitted to parent: %q", r.Body)
		}
	}
}

// ── "Lexically bounded scope" — ScopeSinks ────────────────────────────────

func TestScopedOverrides_ScopeSinks(t *testing.T) {
	// Reset root and the named logger to a known empty sinks state. ScopeSinks
	// inside the snippet swaps in the test's InMemorySink; we just want to
	// make sure the cached "order_service" doesn't carry stale state from
	// earlier tests.
	_ = installCapture(t, "order_service")
	// Now drop the captured sink we just installed — ScopeSinks should be the
	// only path that delivers records to the test's `sink`.
	logger.Get("order_service").SetSinks(nil)
	logger.Get("").SetSinks(nil)

	ctx := context.Background()

	// --- snippet start -----------------------------------------------
	log := logger.Get("order_service")
	sink := logger.NewInMemorySink(100, 1)

	err := log.ScopeSinks(ctx, []logger.Sink{sink}, func(ctx context.Context) error {
		runBusinessLogicScoped(ctx)
		// emits via logger.Get("order_service") in this callback land in sink;
		// other modules calling logger.Get("order_service") inside also emit
		// into sink for the duration of the callback.
		return nil
	})
	if err != nil {
		// handle
		t.Fatalf("ScopeSinks: %v", err)
	}

	// Outside the callback, emits go to the global sinks again.
	records := sink.Records()
	_ = records
	// --- snippet end -------------------------------------------------

	if len(records) == 0 {
		t.Errorf("expected scoped emits to be captured; got 0 records")
	}
}

// runBusinessLogicScoped emits via the same logger name so the docs' claim
// about "any module reaching Logger.get('order_service') during the scope"
// can be verified.
func runBusinessLogicScoped(_ context.Context) {
	logger.Get("order_service").Info("inside scope", logger.Attrs{"order.id": 1234})
}
