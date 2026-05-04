package logger_test

import (
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func TestEncodeTraceIDKnownValue(t *testing.T) {
	tid, err := logger.DecodeTraceID("0af7651916cd43dd8448eb211c80319c")
	if err != nil {
		t.Fatalf("DecodeTraceID: %v", err)
	}
	got, err := logger.EncodeTraceID(tid)
	if err != nil {
		t.Fatalf("EncodeTraceID: %v", err)
	}
	if got != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("EncodeTraceID = %q", got)
	}
}

func TestEncodeTraceIDAllZeros(t *testing.T) {
	got, err := logger.EncodeTraceID(make([]byte, 16))
	if err != nil {
		t.Fatalf("EncodeTraceID: %v", err)
	}
	if got != "00000000000000000000000000000000" {
		t.Fatalf("EncodeTraceID(zeros) = %q", got)
	}
}

func TestEncodeTraceIDNilReturnsEmpty(t *testing.T) {
	got, err := logger.EncodeTraceID(nil)
	if err != nil {
		t.Fatalf("EncodeTraceID(nil): %v", err)
	}
	if got != "" {
		t.Fatalf("EncodeTraceID(nil) = %q, want empty", got)
	}
}

func TestEncodeTraceIDWrongLength(t *testing.T) {
	_, err := logger.EncodeTraceID(make([]byte, 8))
	if err == nil {
		t.Fatalf("EncodeTraceID(8 bytes) expected error")
	}
	if !strings.Contains(err.Error(), "16 bytes") {
		t.Fatalf("error %q does not mention 16 bytes", err)
	}
}

func TestEncodeSpanIDKnownValue(t *testing.T) {
	sid, err := logger.DecodeSpanID("b7ad6b7169203331")
	if err != nil {
		t.Fatalf("DecodeSpanID: %v", err)
	}
	got, err := logger.EncodeSpanID(sid)
	if err != nil {
		t.Fatalf("EncodeSpanID: %v", err)
	}
	if got != "b7ad6b7169203331" {
		t.Fatalf("EncodeSpanID = %q", got)
	}
}

func TestEncodeSpanIDNilReturnsEmpty(t *testing.T) {
	got, err := logger.EncodeSpanID(nil)
	if err != nil || got != "" {
		t.Fatalf("EncodeSpanID(nil) = %q, %v", got, err)
	}
}

func TestEncodeSpanIDWrongLength(t *testing.T) {
	_, err := logger.EncodeSpanID(make([]byte, 16))
	if err == nil {
		t.Fatalf("EncodeSpanID(16 bytes) expected error")
	}
	if !strings.Contains(err.Error(), "8 bytes") {
		t.Fatalf("error %q does not mention 8 bytes", err)
	}
}

func TestRoundtripTraceID(t *testing.T) {
	original := make([]byte, 16)
	for i := range original {
		original[i] = byte(i)
	}
	hexForm, err := logger.EncodeTraceID(original)
	if err != nil {
		t.Fatalf("EncodeTraceID: %v", err)
	}
	decoded, err := logger.DecodeTraceID(hexForm)
	if err != nil {
		t.Fatalf("DecodeTraceID: %v", err)
	}
	if string(decoded) != string(original) {
		t.Fatalf("roundtrip mismatch: %x vs %x", decoded, original)
	}
}

func TestRoundtripSpanID(t *testing.T) {
	original := make([]byte, 8)
	for i := range original {
		original[i] = byte(i)
	}
	hexForm, err := logger.EncodeSpanID(original)
	if err != nil {
		t.Fatalf("EncodeSpanID: %v", err)
	}
	decoded, err := logger.DecodeSpanID(hexForm)
	if err != nil {
		t.Fatalf("DecodeSpanID: %v", err)
	}
	if string(decoded) != string(original) {
		t.Fatalf("roundtrip mismatch: %x vs %x", decoded, original)
	}
}

func TestDecodeTraceIDInvalid(t *testing.T) {
	_, err := logger.DecodeTraceID("deadbeef")
	if err == nil {
		t.Fatalf("expected length error")
	}
	if !strings.Contains(err.Error(), "32 chars") {
		t.Fatalf("error %q does not mention 32 chars", err)
	}

	_, err = logger.DecodeTraceID(strings.Repeat("g", 32))
	if err == nil {
		t.Fatalf("expected hex error")
	}
	if !strings.Contains(err.Error(), "invalid chars") {
		t.Fatalf("error %q does not mention invalid chars", err)
	}
}

func TestDecodeSpanIDInvalid(t *testing.T) {
	_, err := logger.DecodeSpanID("deadbeef")
	if err == nil {
		t.Fatalf("expected length error")
	}
	if !strings.Contains(err.Error(), "16 chars") {
		t.Fatalf("error %q does not mention 16 chars", err)
	}

	_, err = logger.DecodeSpanID(strings.Repeat("z", 16))
	if err == nil {
		t.Fatalf("expected hex error")
	}
}

func TestDecodeTraceIDEmptyReturnsNil(t *testing.T) {
	got, err := logger.DecodeTraceID("")
	if err != nil || got != nil {
		t.Fatalf("DecodeTraceID(\"\") = %v, %v", got, err)
	}
}

func TestDecodeSpanIDEmptyReturnsNil(t *testing.T) {
	got, err := logger.DecodeSpanID("")
	if err != nil || got != nil {
		t.Fatalf("DecodeSpanID(\"\") = %v, %v", got, err)
	}
}
