package logger_test

import (
	"context"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"

	"go.dagstack.dev/logger"
)

func TestActiveTraceContextNoSpan(t *testing.T) {
	tid, sid, flags := logger.ActiveTraceContext(context.Background())
	if tid != nil || sid != nil {
		t.Fatalf("non-nil ids without active span: %v %v", tid, sid)
	}
	if flags != 0 {
		t.Fatalf("flags = %d, want 0", flags)
	}
}

func TestActiveTraceContextNilCtx(t *testing.T) {
	tid, sid, flags := logger.ActiveTraceContext(nil) //nolint:staticcheck // intentionally testing nil ctx
	if tid != nil || sid != nil || flags != 0 {
		t.Fatalf("expected zero values for nil ctx, got %v %v %d", tid, sid, flags)
	}
}

func TestActiveTraceContextWithSpanContext(t *testing.T) {
	traceIDHex := "0af7651916cd43dd8448eb211c80319c"
	spanIDHex := "b7ad6b7169203331"
	tid, err := oteltrace.TraceIDFromHex(traceIDHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	sid, err := oteltrace.SpanIDFromHex(spanIDHex)
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     false,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), sc)

	gotTID, gotSID, flags := logger.ActiveTraceContext(ctx)
	gotTIDHex, _ := logger.EncodeTraceID(gotTID)
	gotSIDHex, _ := logger.EncodeSpanID(gotSID)
	if gotTIDHex != traceIDHex {
		t.Fatalf("trace_id = %q, want %q", gotTIDHex, traceIDHex)
	}
	if gotSIDHex != spanIDHex {
		t.Fatalf("span_id = %q, want %q", gotSIDHex, spanIDHex)
	}
	if flags != 1 {
		t.Fatalf("flags = %d, want 1", flags)
	}
}

func TestDefaultBaggageKeysContainsExpected(t *testing.T) {
	want := map[string]bool{"tenant.id": false, "request.id": false, "user.id": false}
	for _, k := range logger.DefaultBaggageKeys {
		if _, ok := want[k]; ok {
			want[k] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("DefaultBaggageKeys missing %q", k)
		}
	}
}
