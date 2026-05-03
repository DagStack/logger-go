// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/guides/testing.mdx` (Go TabItem).

package docs_examples_test

import (
	"context"
	"testing"

	"go.dagstack.dev/logger"
)

// ── Step 1. "Capture records for one test" — TestOrderPlacementLogsAuditEvent

// placeOrder is a stub that emits the audit record the snippet asserts on.
func placeOrder(_ context.Context, orderID, userID int) {
	logger.Get("order_service.checkout").Info("order placed", logger.Attrs{
		"order.id": orderID,
		"user.id":  userID,
	})
}

// The docs snippet renders a top-level test func; we wrap it as a sub-test.
func TestGuidesTesting_OrderPlacementLogsAuditEvent(t *testing.T) {
	// Reset the global logger config so emits routed via the cached
	// "order_service.checkout" logger don't leak into another test's
	// captures (the registry is global).
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(),
	)

	// --- snippet start -----------------------------------------------
	sink := logger.NewInMemorySink(100, 1)
	log := logger.Get("order_service.checkout")

	err := log.ScopeSinks(context.Background(), []logger.Sink{sink}, func(ctx context.Context) error {
		placeOrder(ctx, 1234, 42)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	records := sink.Records()
	var audit *logger.LogRecord
	for _, r := range records {
		if r.Body == "order placed" {
			audit = r
			break
		}
	}
	if audit == nil {
		t.Fatal("audit record not captured")
	}
	if audit.SeverityText != logger.SeverityTextInfo {
		t.Errorf("severity_text = %q, want INFO", audit.SeverityText)
	}
	if audit.Attributes["order.id"] != 1234 {
		t.Errorf("order.id = %v", audit.Attributes["order.id"])
	}
	// --- snippet end -------------------------------------------------
}

// ── Step 2. "Reusable test fixture" — captureLogs(t) helper ───────────────

// captureLogs is a t.Cleanup-aware helper: it scopes a fresh InMemorySink
// onto "order_service" for the test and restores the original sinks on
// cleanup.
//
// --- snippet start (Step 2 helper) ---------------------------------------
func captureLogs(t *testing.T) *logger.InMemorySink {
	t.Helper()
	sink := logger.NewInMemorySink(1000, 1)
	log := logger.Get("order_service")
	prev := log.EffectiveSinks()
	log.SetSinks([]logger.Sink{sink})
	t.Cleanup(func() {
		log.SetSinks(prev)
	})
	return sink
}

// --- snippet end (Step 2 helper) -----------------------------------------

// placeOrderViaParent emits via the parent name so the captureLogs helper
// (which targets "order_service") sees the records.
func placeOrderViaParent(_ context.Context, orderID, userID int) {
	logger.Get("order_service").Info("order placed", logger.Attrs{
		"order.id": orderID,
		"user.id":  userID,
	})
}

func TestGuidesTesting_AuditTrail(t *testing.T) {
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(),
	)
	// --- snippet start -----------------------------------------------
	sink := captureLogs(t)
	placeOrderViaParent(context.Background(), 1234, 42) // docs: placeOrder(...)
	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].Attributes["order.id"] != 1234 {
		t.Errorf("order.id = %v", records[0].Attributes["order.id"])
	}
	// --- snippet end -------------------------------------------------
}

// ── Step 3. "Asserting on attributes" — Redaction + multi-error ───────────

func TestGuidesTesting_RedactionMasksAPIKeys(t *testing.T) {
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(),
	)
	// --- snippet start -----------------------------------------------
	sink := captureLogs(t)
	logger.Get("order_service").Info("authenticated", logger.Attrs{
		"user.id": 42,
		"api_key": "sk-supersecret",
	})

	record := sink.Records()[0]
	if record.Attributes["api_key"] != logger.RedactedPlaceholder {
		t.Errorf("api_key = %v, want %q", record.Attributes["api_key"], logger.RedactedPlaceholder)
	}
	if record.Attributes["user.id"] != 42 {
		t.Errorf("user.id = %v", record.Attributes["user.id"])
	}
	// --- snippet end -------------------------------------------------
}

// runBusinessLogicWithError emits one ERROR record so the snippet's
// "exactly one error" assertion has something to find.
func runBusinessLogicWithError() {
	log := logger.Get("order_service")
	log.ExceptionCtx(context.Background(), &errOrderValidation{msg: "boom"}, nil, logger.Attrs{
		"order.id": 1234,
	})
}

func TestGuidesTesting_OnlyOneErrorEmitted(t *testing.T) {
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(),
	)
	// --- snippet start -----------------------------------------------
	sink := captureLogs(t)
	runBusinessLogicWithError() // docs: runBusinessLogic()

	var errs []*logger.LogRecord
	for _, r := range sink.Records() {
		if r.SeverityText == logger.SeverityTextError {
			errs = append(errs, r)
		}
	}
	if len(errs) != 1 {
		t.Fatalf("len(errors) = %d, want 1", len(errs))
	}
	if errs[0].Attributes["exception.type"] == nil {
		t.Errorf("exception.type missing")
	}
	// --- snippet end -------------------------------------------------
}

// ── Step 4. "Resetting between assertions" — Clear() ──────────────────────

func runPhaseOne() {
	logger.Get("order_service").Info("phase 1 complete", logger.Attrs{"phase": "one"})
}
func runPhaseTwo() {
	logger.Get("order_service").Info("phase 2 record", logger.Attrs{"phase": "two"})
	logger.Get("order_service").Info("phase 2 record b", logger.Attrs{"phase": "two"})
}

func TestGuidesTesting_PhaseSeparation(t *testing.T) {
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(),
	)
	// --- snippet start -----------------------------------------------
	sink := captureLogs(t)

	runPhaseOne()
	found := false
	for _, r := range sink.Records() {
		if r.Body == "phase 1 complete" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("phase 1 record missing")
	}

	sink.Clear()

	runPhaseTwo()
	for _, r := range sink.Records() {
		if r.Attributes["phase"] != "two" {
			t.Errorf("phase = %v", r.Attributes["phase"])
		}
	}
	// --- snippet end -------------------------------------------------
}

// ── Step 5. "Avoiding capacity overflow" — Capacity() check ───────────────

func indexRepository(_ context.Context) {
	log := logger.Get("indexer")
	for i := 0; i < 5; i++ {
		log.Info("chunk emitted", logger.Attrs{"i": i})
	}
	log.Info("indexing finished", logger.Attrs{"event.name": "completed"})
}

func TestGuidesTesting_HighVolume(t *testing.T) {
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(),
	)
	// --- snippet start -----------------------------------------------
	sink := logger.NewInMemorySink(10_000, 1)
	log := logger.Get("indexer")

	err := log.ScopeSinks(context.Background(), []logger.Sink{sink}, func(ctx context.Context) error {
		indexRepository(ctx)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	records := sink.Records()
	if len(records) > sink.Capacity() {
		t.Errorf("len(records) = %d > capacity %d", len(records), sink.Capacity())
	}
	// --- snippet end -------------------------------------------------

	if sink.Capacity() != 10_000 {
		t.Errorf("Capacity() = %d, want 10000", sink.Capacity())
	}
}
