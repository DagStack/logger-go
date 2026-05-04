package logger

// Attrs is a convenience alias for the per-record attribute map. Per spec
// ADR-0001 §1, attributes is a Map<string, Value> where Value is a recursive
// sum type: string | int | float | bool | nil | map[string]Value | []Value.
//
// In Go we use any (interface{}) as the value type to avoid an unwieldy
// recursive type alias; runtime checks in the wire emitter validate that
// values are JSON-serializable.
type Attrs = map[string]any

// InstrumentationScope describes the logger that emitted a LogRecord.
// Per spec ADR-0001 §4.1 — name + version + optional attrs.
//
// Name matches the logger name (e.g., "dagstack.rag.retriever"); version
// is the semantic version of the package or plugin.
type InstrumentationScope struct {
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	Attributes Attrs  `json:"attributes,omitempty"`
}

// Resource carries process/service/host-level attributes shared across all
// loggers in the same process (OTel Resource). Per spec ADR-0001 §4.2 —
// typical keys: service.name, service.version, deployment.environment,
// host.name, process.pid, telemetry.sdk.{name,version,language}.
type Resource struct {
	Attributes Attrs `json:"attributes,omitempty"`
}

// LogRecord is the OTel Log Data Model v1.24-compatible log record.
//
// Per spec ADR-0001 §1: internal field names match the OTel normative spec
// (TimeUnixNano, ObservedTimeUnixNano, SeverityNumber, SeverityText, Body,
// Attributes, Resource, InstrumentationScope, TraceId as 16 bytes, SpanId
// as 8 bytes, TraceFlags).
//
// Wire serialization (OTLP / JSON) lives in separate functions, see wire.go:
//
//   - dagstack JSON-lines: snake_case keys, hex trace/span ids, raw int
//     timestamps.
//   - OTel JSON (Phase 2+): camelCase keys, stringified int timestamps.
//   - OTLP protobuf (Phase 2+): native OTel wire.
//
// ObservedTimeUnixNano — the sink sets it on ingestion when zero (per spec
// §1 ownership rule).
type LogRecord struct {
	// TimeUnixNano is the emit time in nanoseconds since the Unix epoch.
	TimeUnixNano int64

	// SeverityNumber is the OTel severity in the [1, 24] range.
	SeverityNumber int

	// SeverityText is one of the 6 canonical strings (see CanonicalSeverityTexts).
	SeverityText string

	// Body is the primary message payload. May be a string or a structured
	// value (map/array/scalar) — represented as any.
	Body any

	// Attributes is the per-record key-value context.
	Attributes Attrs

	// InstrumentationScope is the logger self-descriptor (§4.1). May be nil
	// for direct LogRecord construction outside the Logger API.
	InstrumentationScope *InstrumentationScope

	// Resource is the process/service/host attribute set (§4.2). May be nil.
	Resource *Resource

	// TraceID is the W3C Trace Context — 16 random bytes when an active span
	// is present; nil otherwise.
	TraceID []byte

	// SpanID is the W3C Trace Context — 8 random bytes when an active span
	// is present; nil otherwise.
	SpanID []byte

	// TraceFlags is the W3C flags byte (sampled bit etc.). Zero when no
	// active span.
	TraceFlags uint8

	// ObservedTimeUnixNano is the ingest time at the sink, in nanoseconds.
	// The producer leaves this 0; the sink fills it in (per spec §1).
	ObservedTimeUnixNano int64
}
