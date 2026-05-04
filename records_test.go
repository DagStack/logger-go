package logger_test

import (
	"testing"

	"go.dagstack.dev/logger"
)

func TestInstrumentationScopeNameOnly(t *testing.T) {
	s := logger.InstrumentationScope{Name: "dagstack.rag"}
	if s.Name != "dagstack.rag" {
		t.Fatalf("Name = %q", s.Name)
	}
	if s.Version != "" {
		t.Fatalf("Version = %q, want empty", s.Version)
	}
	if s.Attributes != nil {
		t.Fatalf("Attributes non-nil for empty constructor")
	}
}

func TestInstrumentationScopeWithVersionAndAttrs(t *testing.T) {
	s := logger.InstrumentationScope{
		Name:       "dagstack.rag.retriever",
		Version:    "1.4.2",
		Attributes: logger.Attrs{"foo": "bar"},
	}
	if s.Version != "1.4.2" {
		t.Fatalf("Version = %q", s.Version)
	}
	if s.Attributes["foo"] != "bar" {
		t.Fatalf("Attributes[foo] = %v", s.Attributes["foo"])
	}
}

func TestResourceEmpty(t *testing.T) {
	r := logger.Resource{}
	if len(r.Attributes) != 0 {
		t.Fatalf("Attributes non-empty in zero-value Resource")
	}
}

func TestResourceWithAttrs(t *testing.T) {
	r := logger.Resource{Attributes: logger.Attrs{"service.name": "pilot-app"}}
	if r.Attributes["service.name"] != "pilot-app" {
		t.Fatalf("service.name = %v", r.Attributes["service.name"])
	}
}

func TestLogRecordMinimal(t *testing.T) {
	rec := logger.LogRecord{
		TimeUnixNano:   1700000000000000000,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "hello world",
	}
	if rec.SeverityText != "INFO" {
		t.Fatalf("SeverityText = %q", rec.SeverityText)
	}
	if rec.Attributes != nil {
		t.Fatalf("Attributes non-nil for minimal record")
	}
	if rec.InstrumentationScope != nil || rec.Resource != nil {
		t.Fatalf("scope or resource non-nil for minimal record")
	}
	if rec.TraceID != nil || rec.SpanID != nil {
		t.Fatalf("trace/span non-nil for minimal record")
	}
	if rec.TraceFlags != 0 || rec.ObservedTimeUnixNano != 0 {
		t.Fatalf("flags/observed_time non-zero for minimal record")
	}
}

func TestLogRecordFullFields(t *testing.T) {
	scope := &logger.InstrumentationScope{Name: "dagstack.rag", Version: "1.0"}
	resource := &logger.Resource{Attributes: logger.Attrs{"service.name": "pilot-app"}}
	traceID, _ := logger.DecodeTraceID("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := logger.DecodeSpanID("b7ad6b7169203331")

	rec := logger.LogRecord{
		TimeUnixNano:         1700000000000000000,
		SeverityNumber:       17,
		SeverityText:         "ERROR",
		Body:                 map[string]any{"msg": "failure", "code": 500},
		Attributes:           logger.Attrs{"user.id": 42, "request.id": "req-abc"},
		InstrumentationScope: scope,
		Resource:             resource,
		TraceID:              traceID,
		SpanID:               spanID,
		TraceFlags:           1,
		ObservedTimeUnixNano: 1700000000000000123,
	}
	if rec.SeverityText != "ERROR" {
		t.Fatalf("SeverityText = %q", rec.SeverityText)
	}
	if rec.Attributes["user.id"] != 42 {
		t.Fatalf("Attributes[user.id] = %v", rec.Attributes["user.id"])
	}
	if len(rec.TraceID) != 16 {
		t.Fatalf("TraceID length = %d", len(rec.TraceID))
	}
	if len(rec.SpanID) != 8 {
		t.Fatalf("SpanID length = %d", len(rec.SpanID))
	}
}

func TestLogRecordStructuredBody(t *testing.T) {
	rec := logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           map[string]any{"nested": []any{1, 2, map[string]any{"deep": true}}},
	}
	body, ok := rec.Body.(map[string]any)
	if !ok {
		t.Fatalf("Body type = %T, want map[string]any", rec.Body)
	}
	if _, ok := body["nested"].([]any); !ok {
		t.Fatalf("nested type = %T, want []any", body["nested"])
	}
}
