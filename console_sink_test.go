package logger_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func mkRecord(body any, severityNumber int, severityText string, attrs logger.Attrs, scopeName string) *logger.LogRecord {
	rec := &logger.LogRecord{
		TimeUnixNano:   1700000000000000000,
		SeverityNumber: severityNumber,
		SeverityText:   severityText,
		Body:           body,
		Attributes:     attrs,
	}
	if scopeName != "" {
		rec.InstrumentationScope = &logger.InstrumentationScope{Name: scopeName}
	}
	return rec
}

func TestConsoleSinkJSONMode(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &buf, 1)
	sink.Emit(mkRecord("hello", 9, "INFO", nil, "dagstack.rag"))
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output not newline-terminated: %q", out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("re-parse: %v\noutput: %s", err, out)
	}
	if parsed["body"] != "hello" {
		t.Fatalf("body = %v", parsed["body"])
	}
	if parsed["severity_text"] != "INFO" {
		t.Fatalf("severity_text = %v", parsed["severity_text"])
	}
}

func TestConsoleSinkJSONSortedKeys(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &buf, 1)
	sink.Emit(mkRecord("msg", 9, "INFO", nil, "rag"))
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), `{"body":`) {
		t.Fatalf("not canonical-sorted: %q", buf.String())
	}
}

func TestConsoleSinkMultipleRecordsSeparatedByNewlines(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &buf, 1)
	sink.Emit(mkRecord("one", 9, "INFO", nil, "rag"))
	sink.Emit(mkRecord("two", 9, "INFO", nil, "rag"))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), buf.String())
	}
}

func TestConsoleSinkAutoModeNonTTYUsesJSON(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsoleAuto, &buf, 1)
	sink.Emit(mkRecord("hello", 9, "INFO", nil, "rag"))
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("auto on non-TTY did not produce JSON: %v\nout: %s", err, buf.String())
	}
}

func TestConsoleSinkPrettyIncludesSeverity(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(mkRecord("x", 13, "WARN", nil, "rag"))
	if !strings.Contains(buf.String(), "WARN") {
		t.Fatalf("output lacks severity: %q", buf.String())
	}
}

func TestConsoleSinkPrettyIncludesLoggerName(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(mkRecord("x", 9, "INFO", nil, "dagstack.test"))
	if !strings.Contains(buf.String(), "dagstack.test") {
		t.Fatalf("output lacks logger name: %q", buf.String())
	}
}

func TestConsoleSinkPrettyRootLoggerFallback(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(&logger.LogRecord{
		TimeUnixNano:   1700000000000000000,
		SeverityNumber: 9,
		SeverityText:   "INFO",
		Body:           "x",
	})
	if !strings.Contains(buf.String(), "root") {
		t.Fatalf("output lacks root fallback: %q", buf.String())
	}
}

func TestConsoleSinkPrettyAttributesRendered(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(mkRecord("x", 9, "INFO", logger.Attrs{"user.id": 42, "request.id": "abc"}, "rag"))
	out := buf.String()
	if !strings.Contains(out, "user.id=42") {
		t.Fatalf("user.id not rendered: %q", out)
	}
	if !strings.Contains(out, "request.id=abc") {
		t.Fatalf("request.id not rendered: %q", out)
	}
}

func TestConsoleSinkPrettyStringWithSpaceQuoted(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(mkRecord("x", 9, "INFO", logger.Attrs{"msg": "hello world"}, "rag"))
	if !strings.Contains(buf.String(), `"hello world"`) {
		t.Fatalf("quoted string missing: %q", buf.String())
	}
}

func TestConsoleSinkPrettyTimestampISO(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(mkRecord("x", 9, "INFO", nil, "rag"))
	out := buf.String()
	if !strings.Contains(out, "2023-11-14T22:13:20") {
		t.Fatalf("expected ISO timestamp 2023-11-14T22:13:20 in %q", out)
	}
	if !strings.Contains(out, "Z") {
		t.Fatalf("expected Z suffix in timestamp: %q", out)
	}
}

func TestConsoleSinkPrettyStructuredBodyAsJSON(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(mkRecord(map[string]any{"nested": []any{1, 2}}, 9, "INFO", nil, "rag"))
	if !strings.Contains(buf.String(), `{"nested":[1,2]}`) {
		t.Fatalf("structured body not rendered: %q", buf.String())
	}
}

func TestConsoleSinkPrettyScalarTypesInAttributes(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsolePretty, &buf, 1)
	sink.Emit(mkRecord("x", 9, "INFO", logger.Attrs{
		"is_production":  true,
		"debug_mode":     false,
		"optional_value": nil,
	}, "rag"))
	out := buf.String()
	if !strings.Contains(out, "is_production=true") {
		t.Fatalf("bool true not rendered: %q", out)
	}
	if !strings.Contains(out, "debug_mode=false") {
		t.Fatalf("bool false not rendered: %q", out)
	}
	if !strings.Contains(out, "optional_value=null") {
		t.Fatalf("nil not rendered: %q", out)
	}
}

func TestConsoleSinkMinSeverityDropsBelow(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &buf, 9)
	sink.Emit(mkRecord("debug", 5, "DEBUG", nil, "rag"))
	sink.Emit(mkRecord("info", 9, "INFO", nil, "rag"))
	count := strings.Count(buf.String(), "\n")
	if count != 1 {
		t.Fatalf("got %d records, want 1", count)
	}
}

func TestConsoleSinkSupportsSeverity(t *testing.T) {
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &bytes.Buffer{}, 13)
	if sink.SupportsSeverity(9) {
		t.Errorf("supports 9 below threshold 13")
	}
	if !sink.SupportsSeverity(13) {
		t.Errorf("does not support 13 at threshold")
	}
}

func TestConsoleSinkClosePreventsFurtherWrites(t *testing.T) {
	var buf bytes.Buffer
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &buf, 1)
	sink.Emit(mkRecord("before", 9, "INFO", nil, "rag"))
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	sink.Emit(mkRecord("after", 9, "INFO", nil, "rag"))
	count := strings.Count(buf.String(), "\n")
	if count != 1 {
		t.Fatalf("got %d records after close, want 1", count)
	}
}

func TestConsoleSinkCloseIdempotent(t *testing.T) {
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &bytes.Buffer{}, 1)
	if err := sink.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close 2: %v", err)
	}
}

func TestConsoleSinkFlushNoError(t *testing.T) {
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &bytes.Buffer{}, 1)
	if err := sink.Flush(0.1); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestConsoleSinkFlushAfterClose(t *testing.T) {
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &bytes.Buffer{}, 1)
	_ = sink.Close()
	if err := sink.Flush(0); err != nil {
		t.Fatalf("Flush after Close: %v", err)
	}
}

func TestConsoleSinkIDReflectsMode(t *testing.T) {
	cases := map[logger.ConsoleMode]string{
		logger.ConsoleJSON:   "console:json",
		logger.ConsolePretty: "console:pretty",
		logger.ConsoleAuto:   "console:auto",
	}
	for mode, want := range cases {
		got := logger.NewConsoleSink(mode, &bytes.Buffer{}, 1).ID()
		if got != want {
			t.Errorf("ID(%v) = %q, want %q", mode, got, want)
		}
	}
}

func TestConsoleSinkProtocolCompliance(t *testing.T) {
	var s logger.Sink = logger.NewConsoleSink(logger.ConsoleJSON, &bytes.Buffer{}, 1)
	_ = s
}

func TestConsoleSinkNilStreamUsesStderr(t *testing.T) {
	sink := logger.NewConsoleSink(logger.ConsoleJSON, nil, 1)
	if sink == nil {
		t.Fatalf("NewConsoleSink with nil stream returned nil")
	}
}

func TestConsoleSinkEmitNilRecord(t *testing.T) {
	sink := logger.NewConsoleSink(logger.ConsoleJSON, &bytes.Buffer{}, 1)
	sink.Emit(nil) // no panic
}
