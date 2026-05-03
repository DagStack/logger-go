// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/concepts/operations.mdx` (Go TabItem).
//
// The page documents two helpers (logger.operation, logger.emit_event) marked
// "Phase 1 status: not yet shipped". The Go snippet under "manual workaround"
// uses log.Child + manual operation.* attributes — that IS implemented and is
// what we test verbatim.
//
// The snippet imports github.com/google/uuid for operation.id generation; we
// avoid pulling that dependency in by replacing the call with a deterministic
// stand-in fixture string. The substitution is documented inline.

package docs_examples_test

import (
	"testing"

	"go.dagstack.dev/logger"
)

// uuidNewString is a stand-in for uuid.NewString from
// github.com/google/uuid. The docs snippet calls uuid.NewString(); we use a
// fixed string to keep tests deterministic and avoid an extra dep.
func uuidNewString() string { return "00000000-0000-4000-8000-000000000001" }

// ── "Manual workaround" — Child + Info("started") + Info("completed") ─────

func TestOperations_ManualWorkaround(t *testing.T) {
	capture := installCapture(t, "order_service")

	// --- snippet start -----------------------------------------------
	// import (
	//     "github.com/google/uuid"
	//
	//     "go.dagstack.dev/logger"
	// )

	log := logger.Get("order_service")

	opLog := log.Child(logger.Attrs{
		"operation.name": "process_order",
		"operation.id":   uuidNewString(), // docs: uuid.NewString()
		"operation.kind": "lifecycle",
	})
	opLog.Info("started", logger.Attrs{"order.id": 1234})
	opLog.Info("completed", logger.Attrs{
		"operation.status":      "ok",
		"operation.duration_ms": 142,
	})
	// --- snippet end -------------------------------------------------

	records := capture.Records()
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}

	// Both records must carry the bound operation.* attrs from Child(...).
	for i, r := range records {
		if got := r.Attributes["operation.name"]; got != "process_order" {
			t.Errorf("records[%d] operation.name = %v, want process_order", i, got)
		}
		if got := r.Attributes["operation.id"]; got != uuidNewString() {
			t.Errorf("records[%d] operation.id = %v, want stub", i, got)
		}
		if got := r.Attributes["operation.kind"]; got != "lifecycle" {
			t.Errorf("records[%d] operation.kind = %v, want lifecycle", i, got)
		}
	}

	// Per-record specifics.
	if records[0].Body != "started" {
		t.Errorf("records[0].Body = %v, want 'started'", records[0].Body)
	}
	if records[0].Attributes["order.id"] != 1234 {
		t.Errorf("records[0] order.id = %v, want 1234", records[0].Attributes["order.id"])
	}

	if records[1].Body != "completed" {
		t.Errorf("records[1].Body = %v, want 'completed'", records[1].Body)
	}
	if records[1].Attributes["operation.status"] != "ok" {
		t.Errorf("records[1] operation.status = %v, want ok", records[1].Attributes["operation.status"])
	}
	if records[1].Attributes["operation.duration_ms"] != 142 {
		t.Errorf("records[1] operation.duration_ms = %v, want 142", records[1].Attributes["operation.duration_ms"])
	}
}
