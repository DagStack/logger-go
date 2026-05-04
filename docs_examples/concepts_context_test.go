// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/concepts/context.mdx` (Go TabItem).

package docs_examples_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.dagstack.dev/logger"
)

// ── "Setting baggage entries" — baggage.New + ContextWithBaggage ──────────

func TestContext_BaggageContextWithBaggage(t *testing.T) {
	capture := installCapture(t, "docs.examples.context.baggage")
	log := logger.Get("docs.examples.context.baggage")

	// --- snippet start -----------------------------------------------
	// import (
	//     "context"
	//     "go.opentelemetry.io/otel/baggage"
	// )

	member, _ := baggage.NewMember("tenant.id", "acme-corp")
	bag, _ := baggage.New(member)
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	log.InfoCtx(ctx, "processing request", nil)
	// The emitted record carries trace_id / span_id from the active span (if any).
	// Note: Phase 1 Go binding reads trace context from ctx; baggage extraction
	// is gated on a Phase 2 enable flag (see DefaultBaggageKeys).
	// --- snippet end -------------------------------------------------

	records := capture.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	r := records[0]
	if r.Body != "processing request" {
		t.Errorf("body = %v, want 'processing request'", r.Body)
	}
	// Phase 1 caveat — baggage is NOT injected into attributes yet. The docs
	// admonition is explicit about this. Assert the documented behaviour:
	// trace_id is nil here because no span is active, and tenant.id is NOT
	// auto-injected because baggage extraction is Phase 2.
	if r.TraceID != nil {
		t.Errorf("TraceID expected nil (no active span), got %x", r.TraceID)
	}
	if _, ok := r.Attributes["tenant.id"]; ok {
		t.Errorf("tenant.id auto-inject is gated on Phase 2; got %v", r.Attributes["tenant.id"])
	}

	// Confirm DefaultBaggageKeys still lists the conventional allow-list.
	if len(logger.DefaultBaggageKeys) == 0 {
		t.Error("DefaultBaggageKeys should not be empty")
	}
}

// ── Trace context propagation — InfoCtx with an active span ───────────────
//
// While the snippet itself doesn't construct a span (it merely says "if any"),
// the surrounding prose claims trace_id / span_id are auto-injected from the
// active span in ctx. This test confirms that contract for an OTel
// SpanContext explicitly attached to ctx.

func TestContext_InfoCtxInjectsTraceID(t *testing.T) {
	capture := installCapture(t, "docs.examples.context.trace")

	tid, _ := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	sid, _ := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     true,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), sc)

	logger.Get("docs.examples.context.trace").InfoCtx(ctx, "with span", nil)

	r := capture.Records()[0]
	if len(r.TraceID) != 16 {
		t.Errorf("TraceID len = %d, want 16", len(r.TraceID))
	}
	if len(r.SpanID) != 8 {
		t.Errorf("SpanID len = %d, want 8", len(r.SpanID))
	}
	if r.TraceFlags != uint8(oteltrace.FlagsSampled) {
		t.Errorf("TraceFlags = %d, want %d", r.TraceFlags, uint8(oteltrace.FlagsSampled))
	}
}
