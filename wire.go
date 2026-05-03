package logger

// ToDagstackJSONLDict converts a LogRecord into a Go map ready for canonical
// JSON serialization. Per spec ADR-0001 §1 — dagstack JSON-lines wire format
// uses snake_case field names, lowercase hex trace_id/span_id, and integer
// nanoseconds for timestamps.
//
// Empty / zero fields are omitted from the result for cleaner diagnostics:
//
//   - observed_time_unix_nano omitted when zero
//   - attributes omitted when empty
//   - instrumentation_scope omitted when nil
//   - resource omitted when nil or empty
//   - trace_id / span_id omitted when nil
//   - trace_flags omitted when zero
func ToDagstackJSONLDict(record *LogRecord) (map[string]any, error) {
	if record == nil {
		return nil, nil
	}

	out := map[string]any{
		"time_unix_nano":  record.TimeUnixNano,
		"severity_number": record.SeverityNumber,
		"severity_text":   record.SeverityText,
		"body":            record.Body,
	}

	if record.ObservedTimeUnixNano != 0 {
		out["observed_time_unix_nano"] = record.ObservedTimeUnixNano
	}

	if len(record.Attributes) > 0 {
		out["attributes"] = copyAttrs(record.Attributes)
	}

	if record.InstrumentationScope != nil {
		out["instrumentation_scope"] = serializeScope(record.InstrumentationScope)
	}

	if record.Resource != nil && len(record.Resource.Attributes) > 0 {
		out["resource"] = serializeResource(record.Resource)
	}

	if record.TraceID != nil {
		hexStr, err := EncodeTraceID(record.TraceID)
		if err != nil {
			return nil, err
		}
		out["trace_id"] = hexStr
	}
	if record.SpanID != nil {
		hexStr, err := EncodeSpanID(record.SpanID)
		if err != nil {
			return nil, err
		}
		out["span_id"] = hexStr
	}
	if record.TraceFlags != 0 {
		out["trace_flags"] = int(record.TraceFlags)
	}

	return out, nil
}

// ToDagstackJSONL serializes a LogRecord as a single canonical JSON line
// (no trailing newline). Each sink is responsible for adding the LF
// separator between records.
func ToDagstackJSONL(record *LogRecord) (string, error) {
	dict, err := ToDagstackJSONLDict(record)
	if err != nil {
		return "", err
	}
	return CanonicalJSONMarshalString(dict)
}

func serializeScope(scope *InstrumentationScope) map[string]any {
	out := map[string]any{"name": scope.Name}
	if scope.Version != "" {
		out["version"] = scope.Version
	}
	if len(scope.Attributes) > 0 {
		out["attributes"] = copyAttrs(scope.Attributes)
	}
	return out
}

func serializeResource(resource *Resource) map[string]any {
	return map[string]any{
		"attributes": copyAttrs(resource.Attributes),
	}
}

// copyAttrs returns a shallow copy of attrs as map[string]any. Required so
// canonical JSON serialization sees a stable type assertion target.
func copyAttrs(attrs Attrs) map[string]any {
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	return out
}
