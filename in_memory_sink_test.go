package logger_test

import (
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func mkInMemRecord(body string, severityNumber int) *logger.LogRecord {
	return &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: severityNumber,
		SeverityText:   "INFO",
		Body:           body,
	}
}

func TestInMemorySinkEmitCapturesRecord(t *testing.T) {
	sink := logger.NewInMemorySink(10, 1)
	sink.Emit(mkInMemRecord("hello", 9))
	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("got %d, want 1", len(records))
	}
	if records[0].Body != "hello" {
		t.Fatalf("body = %v", records[0].Body)
	}
}

func TestInMemorySinkCapacityBoundedDropsOldest(t *testing.T) {
	sink := logger.NewInMemorySink(3, 1)
	for i := 0; i < 5; i++ {
		sink.Emit(mkInMemRecord(string(rune('0'+i)), 9))
	}
	records := sink.Records()
	if len(records) != 3 {
		t.Fatalf("len = %d, want 3", len(records))
	}
	want := []string{"2", "3", "4"}
	for i, r := range records {
		if r.Body != want[i] {
			t.Fatalf("records[%d].Body = %v, want %v", i, r.Body, want[i])
		}
	}
}

func TestInMemorySinkRecordsReturnsCopy(t *testing.T) {
	sink := logger.NewInMemorySink(10, 1)
	sink.Emit(mkInMemRecord("x", 9))
	snapshot := sink.Records()
	snapshot = snapshot[:0]
	_ = snapshot
	if len(sink.Records()) != 1 {
		t.Fatalf("snapshot mutation leaked into internal storage")
	}
}

func TestInMemorySinkClearEmpties(t *testing.T) {
	sink := logger.NewInMemorySink(10, 1)
	sink.Emit(mkInMemRecord("a", 9))
	sink.Emit(mkInMemRecord("b", 9))
	sink.Clear()
	if len(sink.Records()) != 0 {
		t.Fatalf("after Clear, records = %d", len(sink.Records()))
	}
}

func TestInMemorySinkCloseStopsFurtherEmits(t *testing.T) {
	sink := logger.NewInMemorySink(10, 1)
	sink.Emit(mkInMemRecord("before", 9))
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	sink.Emit(mkInMemRecord("after", 9))
	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("len = %d, want 1", len(records))
	}
	if records[0].Body != "before" {
		t.Fatalf("body = %v", records[0].Body)
	}
}

func TestInMemorySinkFlushIsNoop(t *testing.T) {
	sink := logger.NewInMemorySink(10, 1)
	if err := sink.Flush(0); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := sink.Flush(1.0); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestInMemorySinkIDFormat(t *testing.T) {
	sink := logger.NewInMemorySink(42, 1)
	id := sink.ID()
	if !strings.HasPrefix(id, "in-memory:cap=42#") {
		t.Fatalf("ID = %q (expected prefix 'in-memory:cap=42#')", id)
	}
}

func TestInMemorySinkIDPerInstanceUnique(t *testing.T) {
	a := logger.NewInMemorySink(10, 1)
	b := logger.NewInMemorySink(10, 1)
	if a.ID() == b.ID() {
		t.Fatalf("two InMemorySinks with the same capacity share id %q", a.ID())
	}
}

func TestInMemorySinkCapacityProperty(t *testing.T) {
	sink := logger.NewInMemorySink(77, 1)
	if sink.Capacity() != 77 {
		t.Fatalf("Capacity = %d", sink.Capacity())
	}
}

func TestInMemorySinkMinSeverityDropsBelowThreshold(t *testing.T) {
	sink := logger.NewInMemorySink(10, 9)
	sink.Emit(mkInMemRecord("debug", 5))
	sink.Emit(mkInMemRecord("info", 9))
	sink.Emit(mkInMemRecord("error", 17))
	records := sink.Records()
	got := []string{}
	for _, r := range records {
		got = append(got, r.Body.(string))
	}
	if len(got) != 2 || got[0] != "info" || got[1] != "error" {
		t.Fatalf("got %v, want [info error]", got)
	}
}

func TestInMemorySinkSupportsSeverity(t *testing.T) {
	sink := logger.NewInMemorySink(10, 9)
	if sink.SupportsSeverity(5) {
		t.Errorf("supports 5 below threshold 9")
	}
	if !sink.SupportsSeverity(9) {
		t.Errorf("does not support 9 at threshold")
	}
	if !sink.SupportsSeverity(17) {
		t.Errorf("does not support 17")
	}
}

func TestInMemorySinkProtocolCompliance(t *testing.T) {
	var s logger.Sink = logger.NewInMemorySink(10, 1)
	_ = s
}

func TestInMemorySinkZeroCapacityCoercedToOne(t *testing.T) {
	sink := logger.NewInMemorySink(0, 1)
	if sink.Capacity() != 1 {
		t.Fatalf("zero capacity should coerce to 1, got %d", sink.Capacity())
	}
}

func TestInMemorySinkEmitNil(t *testing.T) {
	sink := logger.NewInMemorySink(10, 1)
	sink.Emit(nil) // no panic
	if len(sink.Records()) != 0 {
		t.Fatalf("nil emit captured")
	}
}
