package logger_test

import (
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func TestCanonicalJSONNull(t *testing.T) {
	got, err := logger.CanonicalJSONMarshalString(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "null" {
		t.Fatalf("got %q", got)
	}
}

func TestCanonicalJSONBooleans(t *testing.T) {
	got, _ := logger.CanonicalJSONMarshalString(true)
	if got != "true" {
		t.Fatalf("true got %q", got)
	}
	got, _ = logger.CanonicalJSONMarshalString(false)
	if got != "false" {
		t.Fatalf("false got %q", got)
	}
}

func TestCanonicalJSONStrings(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", `""`},
		{"hello", `"hello"`},
		{"привет", `"привет"`},
	}
	for _, tc := range cases {
		got, err := logger.CanonicalJSONMarshalString(tc.in)
		if err != nil {
			t.Fatalf("err for %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("input %q got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalJSONIntegers(t *testing.T) {
	cases := map[any]string{
		42:        "42",
		-7:        "-7",
		0:         "0",
		int64(99): "99",
	}
	for in, want := range cases {
		got, _ := logger.CanonicalJSONMarshalString(in)
		if got != want {
			t.Fatalf("input %v got %q want %q", in, got, want)
		}
	}
}

func TestCanonicalJSONFloat(t *testing.T) {
	cases := map[float64]string{
		1.5: "1.5",
		0.1: "0.1",
	}
	for in, want := range cases {
		got, _ := logger.CanonicalJSONMarshalString(in)
		if got != want {
			t.Fatalf("input %v got %q want %q", in, got, want)
		}
	}
}

func TestCanonicalJSONNegativeZeroNormalised(t *testing.T) {
	got, err := logger.CanonicalJSONMarshalString(-0.0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "0.0" {
		t.Fatalf("got %q", got)
	}
}

func TestCanonicalJSONNaNRejected(t *testing.T) {
	nan := nanFloat()
	_, err := logger.CanonicalJSONMarshalString(nan)
	if err == nil {
		t.Fatalf("expected error for NaN")
	}
	if !strings.Contains(err.Error(), "NaN") {
		t.Fatalf("error %q does not mention NaN", err)
	}
}

func TestCanonicalJSONInfinityRejected(t *testing.T) {
	for _, v := range []float64{infFloat(), -infFloat()} {
		_, err := logger.CanonicalJSONMarshalString(v)
		if err == nil {
			t.Fatalf("expected error for Inf %v", v)
		}
		if !strings.Contains(err.Error(), "Infinity") {
			t.Fatalf("error %q does not mention Infinity", err)
		}
	}
}

func TestCanonicalJSONEmptyContainers(t *testing.T) {
	got, _ := logger.CanonicalJSONMarshalString([]any{})
	if got != "[]" {
		t.Fatalf("empty array got %q", got)
	}
	got, _ = logger.CanonicalJSONMarshalString(map[string]any{})
	if got != "{}" {
		t.Fatalf("empty object got %q", got)
	}
}

func TestCanonicalJSONKeysSorted(t *testing.T) {
	got, _ := logger.CanonicalJSONMarshalString(map[string]any{"b": 1, "a": 2})
	if got != `{"a":2,"b":1}` {
		t.Fatalf("got %q", got)
	}
}

func TestCanonicalJSONKeysSortedRecursively(t *testing.T) {
	got, _ := logger.CanonicalJSONMarshalString(map[string]any{
		"outer": map[string]any{"z": 1, "a": 2},
	})
	if got != `{"outer":{"a":2,"z":1}}` {
		t.Fatalf("got %q", got)
	}
}

func TestCanonicalJSONArrayPreservesOrder(t *testing.T) {
	got, _ := logger.CanonicalJSONMarshalString([]any{3, 1, 2})
	if got != "[3,1,2]" {
		t.Fatalf("got %q", got)
	}
}

func TestCanonicalJSONUnicodeKeysSortByCodepoint(t *testing.T) {
	got, _ := logger.CanonicalJSONMarshalString(map[string]any{"я": 1, "a": 2})
	if got != `{"a":2,"я":1}` {
		t.Fatalf("got %q", got)
	}
}

func TestCanonicalJSONNoWhitespace(t *testing.T) {
	got, _ := logger.CanonicalJSONMarshalString(map[string]any{
		"a": []any{1, 2, map[string]any{"b": "c"}},
	})
	if strings.ContainsAny(got, " \n\t") {
		t.Fatalf("contains whitespace: %q", got)
	}
}

func TestCanonicalJSONDeterminism(t *testing.T) {
	a := map[string]any{"x": 1, "y": 2, "z": 3}
	b := map[string]any{"z": 3, "x": 1, "y": 2}
	sa, _ := logger.CanonicalJSONMarshalString(a)
	sb, _ := logger.CanonicalJSONMarshalString(b)
	if sa != sb {
		t.Fatalf("non-deterministic: %q vs %q", sa, sb)
	}
}

// helpers — produce special floats without compile-time literal division.
// S3 regression: surrogate-pair keys sort by UTF-16 code units, not by
// UTF-8 byte order. 💎 (U+1F48E, surrogates D83D DC8E) sorts AFTER
// 🍕 (U+1F355, surrogates D83C DF55) because the high surrogate D83C
// < D83D. In UTF-8 byte order it would have been the opposite (💎 first
// because 92 8E < 8D 95 in the trailing bytes). Cross-binding parity:
// matches logger-typescript native Object.keys().sort() and
// logger-python's RFC 8785-conformant sort.
func TestCanonicalJSONKeysSortedByUTF16(t *testing.T) {
	got, err := logger.CanonicalJSONMarshalString(map[string]any{
		"aa": 1,
		"💎":  2, // U+1F48E
		"ab": 3,
		"äz": 4,
		"🍕":  5, // U+1F355
	})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	want := `{"aa":1,"ab":3,"äz":4,"🍕":5,"💎":2}`
	if got != want {
		t.Fatalf("UTF-16 sort mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func nanFloat() float64 {
	zero := 0.0
	return zero / zero
}

func infFloat() float64 {
	one := 1.0
	zero := 0.0
	return one / zero
}
