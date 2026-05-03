package logger

import (
	"encoding/hex"
	"fmt"
)

const (
	traceIDBytes = 16
	spanIDBytes  = 8
)

// EncodeTraceID encodes a 16-byte trace_id as a 32-character lowercase hex
// string. Per spec ADR-0001 §1 — wire-format encoding for dagstack JSON-lines
// and OTel JSON. Returns an empty string when traceID is nil.
func EncodeTraceID(traceID []byte) (string, error) {
	if traceID == nil {
		return "", nil
	}
	if len(traceID) != traceIDBytes {
		return "", fmt.Errorf("trace_id must be %d bytes, got %d", traceIDBytes, len(traceID))
	}
	return hex.EncodeToString(traceID), nil
}

// EncodeSpanID encodes an 8-byte span_id as a 16-character lowercase hex
// string. Returns an empty string when spanID is nil.
func EncodeSpanID(spanID []byte) (string, error) {
	if spanID == nil {
		return "", nil
	}
	if len(spanID) != spanIDBytes {
		return "", fmt.Errorf("span_id must be %d bytes, got %d", spanIDBytes, len(spanID))
	}
	return hex.EncodeToString(spanID), nil
}

// DecodeTraceID decodes a 32-character hex string into a 16-byte trace_id.
// Returns nil when hexStr is empty.
func DecodeTraceID(hexStr string) ([]byte, error) {
	if hexStr == "" {
		return nil, nil
	}
	if len(hexStr) != traceIDBytes*2 {
		return nil, fmt.Errorf("trace_id hex must be %d chars, got %d", traceIDBytes*2, len(hexStr))
	}
	out, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("trace_id hex %q contains invalid chars: %w", hexStr, err)
	}
	return out, nil
}

// DecodeSpanID decodes a 16-character hex string into an 8-byte span_id.
// Returns nil when hexStr is empty.
func DecodeSpanID(hexStr string) ([]byte, error) {
	if hexStr == "" {
		return nil, nil
	}
	if len(hexStr) != spanIDBytes*2 {
		return nil, fmt.Errorf("span_id hex must be %d chars, got %d", spanIDBytes*2, len(hexStr))
	}
	out, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("span_id hex %q contains invalid chars: %w", hexStr, err)
	}
	return out, nil
}
