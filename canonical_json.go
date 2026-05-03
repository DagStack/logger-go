package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf16"
)

// CanonicalJSONMarshal serializes a value into a canonical JSON byte slice
// per RFC 8785 subset (config-spec §9.1.1).
//
// Rules:
//   - Sorted object keys (lexicographic UTF-16 code-unit order per RFC 8785
//     §3.2.3 — matches the cross-binding canonical sort applied by
//     dagstack-logger Python and @dagstack/logger TypeScript).
//   - No whitespace except inside strings.
//   - Integers without a decimal point ("1"); floats use shortest round-trip.
//   - "-0.0" → "0.0" (RFC 8785 §3.2.2.3).
//   - NaN / ±Infinity → error.
//   - Non-string map keys → error.
//   - UTF-8 strings pass through unescaped (HTML-safe escaping disabled).
//
// The implementation mirrors logger-python's canonical_json.canonical_json_dumps
// to guarantee byte-identical output across bindings.
func CanonicalJSONMarshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// CanonicalJSONMarshalString is a convenience wrapper that returns the result
// as a string.
func CanonicalJSONMarshalString(v any) (string, error) {
	b, err := CanonicalJSONMarshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case string:
		return writeCanonicalString(buf, x)
	case int:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int8:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int16:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int32:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int64:
		buf.WriteString(strconv.FormatInt(x, 10))
		return nil
	case uint:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint64:
		buf.WriteString(strconv.FormatUint(x, 10))
		return nil
	case float32:
		return writeCanonicalFloat(buf, float64(x))
	case float64:
		return writeCanonicalFloat(buf, x)
	case []any:
		return writeCanonicalArray(buf, x)
	case map[string]any:
		return writeCanonicalObject(buf, x)
	default:
		return fmt.Errorf("canonical json: unsupported type %T", v)
	}
}

func writeCanonicalString(buf *bytes.Buffer, s string) error {
	// encoding/json.Encoder produces well-formed JSON strings with the same
	// escapes RFC 8785 requires; only the HTML-safe escaping (<, >, &) needs
	// to be disabled because the spec output is not HTML-bound.
	var inner bytes.Buffer
	enc := json.NewEncoder(&inner)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return fmt.Errorf("canonical json: encode string: %w", err)
	}
	out := inner.Bytes()
	// Encoder appends a trailing newline; strip it.
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	buf.Write(out)
	return nil
}

func writeCanonicalFloat(buf *bytes.Buffer, f float64) error {
	if math.IsNaN(f) {
		return fmt.Errorf("canonical json: NaN not allowed")
	}
	if math.IsInf(f, 0) {
		return fmt.Errorf("canonical json: Infinity not allowed")
	}
	if f == 0 {
		// Normalize -0.0 → 0.0 (RFC 8785 §3.2.2.3). Use 0.0 rendering
		// (with a fractional component) to match logger-python output.
		buf.WriteString("0.0")
		return nil
	}
	// strconv.FormatFloat with -1 precision returns the shortest round-trip;
	// 'g' adapts between fixed and exponential. We force 'g' style with
	// ensured fractional component so integer-valued floats render as
	// "1.0" rather than "1".
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !containsFractionalRune(s) {
		s += ".0"
	}
	buf.WriteString(s)
	return nil
}

func containsFractionalRune(s string) bool {
	for _, r := range s {
		if r == '.' || r == 'e' || r == 'E' {
			return true
		}
	}
	return false
}

func writeCanonicalArray(buf *bytes.Buffer, a []any) error {
	buf.WriteByte('[')
	for i, item := range a {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := writeCanonical(buf, item); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

// sortKeysUTF16 sorts keys lexicographically by UTF-16 code-unit sequence
// (RFC 8785 §3.2.3). This differs from sort.Strings (UTF-8 byte order) on
// any key containing characters outside the Basic Multilingual Plane —
// emoji, Han ideographs ≥ U+10000, etc. Keeping byte order would diverge
// from logger-python and logger-typescript on such keys, breaking the
// cross-binding wire-byte parity guarantee.
func sortKeysUTF16(keys []string) {
	sort.SliceStable(keys, func(i, j int) bool {
		a := utf16.Encode([]rune(keys[i]))
		b := utf16.Encode([]rune(keys[j]))
		minLen := len(a)
		if len(b) < minLen {
			minLen = len(b)
		}
		for k := 0; k < minLen; k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
}

func writeCanonicalObject(buf *bytes.Buffer, m map[string]any) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortKeysUTF16(keys)

	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := writeCanonicalString(buf, k); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := writeCanonical(buf, m[k]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}
