package logger

import (
	"context"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// DefaultBaggageKeys is the allow-list of W3C Baggage entries auto-injected
// into LogRecord attributes per spec ADR-0001 §3.4. Other baggage entries
// are skipped to avoid leaking arbitrary cross-service context into logs.
//
// Phase 1 delivers the trace-context portion only; baggage extraction is
// gated on availability of the OTel baggage API and can be enabled in a
// future minor release without breaking the surface.
var DefaultBaggageKeys = []string{"tenant.id", "request.id", "user.id"}

// ActiveTraceContext extracts the active OTel SpanContext from ctx and
// returns its (TraceID, SpanID, TraceFlags) triple ready for LogRecord
// fields. When no valid span context is present, returns (nil, nil, 0).
//
// Per spec §3.4 — the binding MUST use the OTel Context API as the source
// of trace state, not its own context implementation.
func ActiveTraceContext(ctx context.Context) (traceID, spanID []byte, traceFlags uint8) {
	if ctx == nil {
		return nil, nil, 0
	}
	span := oteltrace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return nil, nil, 0
	}
	tid := sc.TraceID()
	sid := sc.SpanID()
	traceID = make([]byte, traceIDBytes)
	spanID = make([]byte, spanIDBytes)
	copy(traceID, tid[:])
	copy(spanID, sid[:])
	traceFlags = uint8(sc.TraceFlags())
	return traceID, spanID, traceFlags
}
