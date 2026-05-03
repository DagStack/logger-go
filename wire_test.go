package logger_test

import (
	"encoding/json"
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func TestWireMinimalRequiredFieldsOnly(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   1700000000000000000,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "hello",
	}
	d, err := logger.ToDagstackJSONLDict(rec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := map[string]any{
		"time_unix_nano":  int64(1700000000000000000),
		"severity_number": 9,
		"severity_text":   "INFO",
		"body":            "hello",
	}
	if len(d) != len(want) {
		t.Fatalf("dict len = %d, want %d (got %v)", len(d), len(want), d)
	}
	for k, v := range want {
		if d[k] != v {
			t.Fatalf("key %q: got %v (%T), want %v (%T)", k, d[k], d[k], v, v)
		}
	}
}

func TestWireOmitsZeroValueOptionalFields(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
	}
	d, err := logger.ToDagstackJSONLDict(rec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, key := range []string{
		"trace_id", "span_id", "instrumentation_scope",
		"resource", "attributes", "observed_time_unix_nano", "trace_flags",
	} {
		if _, ok := d[key]; ok {
			t.Errorf("key %q present, expected omitted", key)
		}
	}
}

func TestWireTraceSpanEmittedAsHex(t *testing.T) {
	tid, _ := logger.DecodeTraceID("0af7651916cd43dd8448eb211c80319c")
	sid, _ := logger.DecodeSpanID("b7ad6b7169203331")
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
		TraceID:        tid,
		SpanID:         sid,
		TraceFlags:     1,
	}
	d, err := logger.ToDagstackJSONLDict(rec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d["trace_id"] != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("trace_id = %v", d["trace_id"])
	}
	if d["span_id"] != "b7ad6b7169203331" {
		t.Fatalf("span_id = %v", d["span_id"])
	}
	if d["trace_flags"] != 1 {
		t.Fatalf("trace_flags = %v", d["trace_flags"])
	}
}

func TestWireTraceFlagsZeroOmitted(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	if _, ok := d["trace_flags"]; ok {
		t.Fatalf("trace_flags present for default record")
	}
}

func TestWireAttributesPopulated(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
		Attributes:     logger.Attrs{"user.id": 42, "request.id": "req-abc"},
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	got, ok := d["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes type = %T", d["attributes"])
	}
	if got["user.id"] != 42 {
		t.Fatalf("user.id = %v", got["user.id"])
	}
}

func TestWireEmptyAttributesOmitted(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
		Attributes:     logger.Attrs{},
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	if _, ok := d["attributes"]; ok {
		t.Fatalf("empty attributes not omitted")
	}
}

func TestWireInstrumentationScopeEmitted(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:         0,
		SeverityNumber:       9,
		SeverityText:         "INFO",
		Body:                 "x",
		InstrumentationScope: &logger.InstrumentationScope{Name: "dagstack.rag", Version: "1.0.0"},
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	scope, ok := d["instrumentation_scope"].(map[string]any)
	if !ok {
		t.Fatalf("scope type = %T", d["instrumentation_scope"])
	}
	if scope["name"] != "dagstack.rag" || scope["version"] != "1.0.0" {
		t.Fatalf("scope = %v", scope)
	}
}

func TestWireScopeWithoutVersion(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:         0,
		SeverityNumber:       9,
		SeverityText:         "INFO",
		Body:                 "x",
		InstrumentationScope: &logger.InstrumentationScope{Name: "root"},
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	scope := d["instrumentation_scope"].(map[string]any)
	if _, ok := scope["version"]; ok {
		t.Fatalf("version present on scope without version")
	}
}

func TestWireResourceEmitted(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
		Resource:       &logger.Resource{Attributes: logger.Attrs{"service.name": "pilot-app"}},
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	res := d["resource"].(map[string]any)
	attrs := res["attributes"].(map[string]any)
	if attrs["service.name"] != "pilot-app" {
		t.Fatalf("service.name = %v", attrs["service.name"])
	}
}

func TestWireEmptyResourceOmitted(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
		Resource:       &logger.Resource{},
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	if _, ok := d["resource"]; ok {
		t.Fatalf("empty resource not omitted")
	}
}

func TestWireObservedTimeEmittedWhenSet(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:         100,
		SeverityNumber:       9,
		SeverityText:         "INFO",
		Body:                 "x",
		ObservedTimeUnixNano: 200,
	}
	d, _ := logger.ToDagstackJSONLDict(rec)
	if d["observed_time_unix_nano"] != int64(200) {
		t.Fatalf("observed_time_unix_nano = %v", d["observed_time_unix_nano"])
	}
}

func TestWireOutputIsCanonicalJSON(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   100,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "hi",
	}
	got, err := logger.ToDagstackJSONL(rec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := `{"body":"hi","severity_number":9,"severity_text":"INFO","time_unix_nano":100}`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestWireOutputRoundTripsViaJSONParse(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           map[string]any{"nested": []any{1, 2, 3}},
		Attributes:     logger.Attrs{"k": "v"},
	}
	s, err := logger.ToDagstackJSONL(rec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	body := parsed["body"].(map[string]any)
	nested := body["nested"].([]any)
	if len(nested) != 3 || int(nested[0].(float64)) != 1 {
		t.Fatalf("nested mismatch: %v", nested)
	}
}

func TestWireNoTrailingNewline(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
	}
	got, _ := logger.ToDagstackJSONL(rec)
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("trailing newline in %q", got)
	}
}

func TestWireDeterministicOutput(t *testing.T) {
	rec := &logger.LogRecord{
		TimeUnixNano:   100,
		SeverityNumber: 17,
		SeverityText:   "ERROR",
		Body:           "x",
		Attributes:     logger.Attrs{"z": 1, "a": 2},
	}
	a, _ := logger.ToDagstackJSONL(rec)
	b, _ := logger.ToDagstackJSONL(rec)
	if a != b {
		t.Fatalf("not deterministic:\n%q\n%q", a, b)
	}
}

func TestWireNilRecord(t *testing.T) {
	d, err := logger.ToDagstackJSONLDict(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("expected nil dict, got %v", d)
	}
}
