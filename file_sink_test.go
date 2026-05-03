package logger_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func mkFileRecord(body string, severityNumber int, severityText string) *logger.LogRecord {
	return &logger.LogRecord{
		TimeUnixNano:   0,
		SeverityNumber: severityNumber,
		SeverityText:   severityText,
		Body:           body,
	}
}

func TestFileSinkWritesJSONLToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	sink.Emit(mkFileRecord("hello", 9, "INFO"))
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["body"] != "hello" {
		t.Fatalf("body = %v", parsed["body"])
	}
}

func TestFileSinkMultipleRecordsMultipleLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	for i := 0; i < 3; i++ {
		sink.Emit(mkFileRecord("msg-"+string(rune('0'+i)), 9, "INFO"))
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	content, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
}

func TestFileSinkIDIncludesPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	defer sink.Close()
	abs, _ := filepath.Abs(path)
	if sink.ID() != "file:"+abs {
		t.Fatalf("ID = %q, want file:%s", sink.ID(), abs)
	}
}

func TestFileSinkRotationBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 200, 2, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	for i := 0; i < 20; i++ {
		sink.Emit(mkFileRecord("record-"+string(rune('a'+i%26)), 9, "INFO"))
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	backups := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "log.jsonl.") {
			backups++
		}
	}
	if backups < 1 {
		t.Fatalf("expected at least 1 backup, got %d", backups)
	}
	if backups > 2 {
		t.Fatalf("expected at most 2 backups (keep=2), got %d", backups)
	}
}

func TestFileSinkMinSeverityDropsBelow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 13)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	sink.Emit(mkFileRecord("debug", 5, "DEBUG"))
	sink.Emit(mkFileRecord("info", 9, "INFO"))
	sink.Emit(mkFileRecord("warn", 13, "WARN"))
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	content, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	var parsed map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &parsed)
	if parsed["body"] != "warn" {
		t.Fatalf("body = %v", parsed["body"])
	}
}

func TestFileSinkClosePreventsFurtherWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	sink.Emit(mkFileRecord("before", 9, "INFO"))
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	sink.Emit(mkFileRecord("after", 9, "INFO"))
	content, _ := os.ReadFile(path)
	if strings.Contains(string(content), "after") {
		t.Fatalf("emit after close persisted")
	}
}

func TestFileSinkCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close 2: %v", err)
	}
}

func TestFileSinkFlush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	sink.Emit(mkFileRecord("x", 9, "INFO"))
	if err := sink.Flush(1); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestFileSinkProtocolCompliance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	sink, err := logger.NewFileSink(path, 0, 0, 1)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	defer sink.Close()
	var s logger.Sink = sink
	_ = s
}

func TestFileSinkOpenError(t *testing.T) {
	// Path that cannot be opened — directory does not exist.
	_, err := logger.NewFileSink("/nonexistent/dir/path/log.jsonl", 0, 0, 1)
	if err == nil {
		t.Fatalf("expected error for nonexistent dir")
	}
}
