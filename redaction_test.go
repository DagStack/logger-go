package logger_test

import (
	"testing"

	"go.dagstack.dev/logger"
)

func TestIsSecretKeySuffixes(t *testing.T) {
	cases := map[string]bool{
		"api_key":         true,
		"OPENAI_API_KEY":  true,
		"client_secret":   true,
		"CLIENT_SECRET":   true,
		"access_token":    true,
		"db_password":     true,
		"my_passphrase":   true,
		"app_credentials": true,
		"user.id":         false,
		"request.id":      false,
		"model":           false,
		"temperature":     false,
	}
	for input, want := range cases {
		got := logger.IsSecretKey(input, nil)
		if got != want {
			t.Errorf("IsSecretKey(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestIsSecretKeyCustomSuffixes(t *testing.T) {
	if !logger.IsSecretKey("app_hash", []string{"_hash"}) {
		t.Errorf("custom suffix not respected")
	}
	if logger.IsSecretKey("app_hash", []string{"_secret"}) {
		t.Errorf("non-matching custom suffix matched")
	}
}

func TestRedactAttributesMasksSecretValues(t *testing.T) {
	in := logger.Attrs{"api_key": "sk-123", "model": "gpt-4"}
	out := logger.RedactAttributes(in, nil)
	if out["api_key"] != logger.RedactedPlaceholder {
		t.Fatalf("api_key not masked: %v", out["api_key"])
	}
	if out["model"] != "gpt-4" {
		t.Fatalf("model unexpectedly altered: %v", out["model"])
	}
}

func TestRedactAttributesReturnsCopy(t *testing.T) {
	in := logger.Attrs{"api_key": "sk-123"}
	_ = logger.RedactAttributes(in, nil)
	if in["api_key"] != "sk-123" {
		t.Fatalf("input mutated: %v", in["api_key"])
	}
}

func TestRedactAttributesRecursiveNested(t *testing.T) {
	in := logger.Attrs{
		"outer":  "fine",
		"nested": map[string]any{"db_password": "hunter2", "safe": "ok"},
	}
	out := logger.RedactAttributes(in, nil)
	if out["outer"] != "fine" {
		t.Fatalf("outer altered")
	}
	nested, ok := out["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested type %T", out["nested"])
	}
	if nested["db_password"] != logger.RedactedPlaceholder {
		t.Fatalf("nested.db_password not masked: %v", nested["db_password"])
	}
	if nested["safe"] != "ok" {
		t.Fatalf("nested.safe altered")
	}
}

func TestRedactAttributesDeepNesting(t *testing.T) {
	in := logger.Attrs{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{"my_token": "secret"},
			},
		},
	}
	out := logger.RedactAttributes(in, nil)
	a := out["a"].(map[string]any)
	b := a["b"].(map[string]any)
	c := b["c"].(map[string]any)
	if c["my_token"] != logger.RedactedPlaceholder {
		t.Fatalf("deep secret not masked: %v", c["my_token"])
	}
}

func TestRedactAttributesDefaultSuffixesCompleteSet(t *testing.T) {
	got := map[string]bool{}
	for _, s := range logger.DefaultSecretSuffixes {
		got[s] = true
	}
	want := []string{"_key", "_secret", "_token", "_password", "_passphrase", "_credentials"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing default suffix %q", w)
		}
	}
}

func TestRedactAttributesCaseInsensitiveMatching(t *testing.T) {
	out := logger.RedactAttributes(logger.Attrs{"API_KEY": "sk-123"}, nil)
	if out["API_KEY"] != logger.RedactedPlaceholder {
		t.Fatalf("uppercase API_KEY not masked")
	}
}

func TestRedactAttributesNilInput(t *testing.T) {
	if logger.RedactAttributes(nil, nil) != nil {
		t.Fatalf("nil input returned non-nil")
	}
}

// Regression for spec §10.2 list-of-maps gap (architect review S8):
// a secret key buried inside a list-of-maps must still be masked.
func TestRedactAttributesRecursesIntoListOfMaps(t *testing.T) {
	in := logger.Attrs{
		"events": []any{
			map[string]any{"type": "login", "user_password": "hunter2"},
			map[string]any{"type": "exchange", "api_key": "sk-secret"},
		},
	}
	out := logger.RedactAttributes(in, nil)
	events, ok := out["events"].([]any)
	if !ok {
		t.Fatalf("events type %T", out["events"])
	}
	if len(events) != 2 {
		t.Fatalf("events len %d", len(events))
	}
	first, ok := events[0].(map[string]any)
	if !ok {
		t.Fatalf("events[0] type %T", events[0])
	}
	if first["user_password"] != logger.RedactedPlaceholder {
		t.Fatalf("events[0].user_password not masked: %v", first["user_password"])
	}
	if first["type"] != "login" {
		t.Fatalf("events[0].type altered: %v", first["type"])
	}
	second, ok := events[1].(map[string]any)
	if !ok {
		t.Fatalf("events[1] type %T", events[1])
	}
	if second["api_key"] != logger.RedactedPlaceholder {
		t.Fatalf("events[1].api_key not masked: %v", second["api_key"])
	}
}

func TestRedactAttributesLeavesPrimitiveListUntouched(t *testing.T) {
	in := logger.Attrs{
		"tags": []any{"alpha", "beta"},
		"nums": []any{1, 2, 3},
	}
	out := logger.RedactAttributes(in, nil)
	tags, ok := out["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "alpha" || tags[1] != "beta" {
		t.Fatalf("tags altered: %v", out["tags"])
	}
	nums, ok := out["nums"].([]any)
	if !ok || len(nums) != 3 || nums[0] != 1 {
		t.Fatalf("nums altered: %v", out["nums"])
	}
}

// Whole-value-mask wins: a list value placed under a secret key is masked
// as a whole, not recursed. Architect-review explicit invariant.
func TestRedactAttributesSecretKeyWithListValueMasksWhole(t *testing.T) {
	in := logger.Attrs{"api_key": []any{"a", "b"}}
	out := logger.RedactAttributes(in, nil)
	if out["api_key"] != logger.RedactedPlaceholder {
		t.Fatalf("whole-list under secret key not masked: %v", out["api_key"])
	}
}

// Whole-value-mask wins for map values under a secret key as well.
func TestRedactAttributesSecretKeyWithMapValueMasksWhole(t *testing.T) {
	in := logger.Attrs{"client_secret": map[string]any{"x": "y"}}
	out := logger.RedactAttributes(in, nil)
	if out["client_secret"] != logger.RedactedPlaceholder {
		t.Fatalf("whole-map under secret key not masked: %v", out["client_secret"])
	}
}

// A list mixing maps with primitives — maps are recursed, primitives pass
// through unchanged.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Regression for architect review M-1 (M3 impl): a child logger inherits
// the disable-all state of its root via the explicit-flag path, not via
// nil-vs-empty propagation. Without `suffixesExplicit`, the empty slice
// could accidentally collapse with the "no override" fallback during
// EffectiveSecretSuffixes resolution.
func TestEffectiveSecretSuffixes_DisableAllInheritedByChild(t *testing.T) {
	logger.ResetRegistryForTests()
	root := logger.Get("")
	root.SetRedactionSuffixes([]string{}) // explicit empty, disable-all
	got := logger.Get("dagstack.rag").EffectiveSecretSuffixes()
	if len(got) != 0 {
		t.Fatalf("child should observe disable-all (empty); got %v", got)
	}
	// And a fresh child after Reset gets the default fallback, not the
	// previous explicit empty.
	logger.ResetRegistryForTests()
	got2 := logger.Get("dagstack.rag").EffectiveSecretSuffixes()
	if len(got2) != len(logger.DefaultSecretSuffixes) {
		t.Fatalf("after Reset, child should fall back to DefaultSecretSuffixes; got %v", got2)
	}
}

func TestBuildEffectiveSuffixes_AdditiveDefault(t *testing.T) {
	got := logger.BuildEffectiveSuffixes(logger.RedactionConfig{
		ExtraSuffixes: []string{"_apikey", "_x_internal_token"},
	})
	want := []string{
		"_key", "_secret", "_token", "_password", "_passphrase", "_credentials",
		"_apikey", "_x_internal_token",
	}
	if !equalStringSlices(got, want) {
		t.Fatalf("Additive merge mismatch:\n got=%v\nwant=%v", got, want)
	}
}

func TestBuildEffectiveSuffixes_ReplaceDefaults(t *testing.T) {
	got := logger.BuildEffectiveSuffixes(logger.RedactionConfig{
		ExtraSuffixes:   []string{"_password"},
		ReplaceDefaults: true,
	})
	if !equalStringSlices(got, []string{"_password"}) {
		t.Fatalf("Replace mode should drop base set; got %v", got)
	}
}

func TestBuildEffectiveSuffixes_ReplaceWithEmptyDisablesAll(t *testing.T) {
	got := logger.BuildEffectiveSuffixes(logger.RedactionConfig{ReplaceDefaults: true})
	if len(got) != 0 {
		t.Fatalf("Replace + empty extras should disable all redaction; got %v", got)
	}
}

func TestBuildEffectiveSuffixes_DeduplicatesAndLowercases(t *testing.T) {
	got := logger.BuildEffectiveSuffixes(logger.RedactionConfig{
		ExtraSuffixes: []string{"_apikey", "_apikey", "_KEY"},
	})
	// `_KEY` is invalid for Validate, but BuildEffective lowercases (defensive).
	// `_apikey` deduplicates; `_key` already in defaults — also deduplicates.
	want := []string{
		"_key", "_secret", "_token", "_password", "_passphrase", "_credentials",
		"_apikey",
	}
	if !equalStringSlices(got, want) {
		t.Fatalf("Dedup/lowercase mismatch:\n got=%v\nwant=%v", got, want)
	}
}

func TestValidateRedactionConfig_RejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"empty_string": "",
		"whitespace":   "_my secret",
		"uppercase":    "_APIKEY",
		"non_ascii":    "_кей",
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			err := logger.ValidateRedactionConfig(logger.RedactionConfig{
				ExtraSuffixes: []string{bad},
			})
			if err == nil {
				t.Fatalf("expected validation error for %q", bad)
			}
		})
	}
}

func TestValidateRedactionConfig_AcceptsValid(t *testing.T) {
	err := logger.ValidateRedactionConfig(logger.RedactionConfig{
		ExtraSuffixes: []string{"_apikey", "_x_internal_token"},
	})
	if err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestRedactAttributesMixedListOfMapAndPrimitives(t *testing.T) {
	in := logger.Attrs{
		"events": []any{
			map[string]any{"api_key": "sk-1"},
			"non-map-item",
			42,
		},
	}
	out := logger.RedactAttributes(in, nil)
	events, ok := out["events"].([]any)
	if !ok || len(events) != 3 {
		t.Fatalf("events shape: %v", out["events"])
	}
	first, ok := events[0].(map[string]any)
	if !ok || first["api_key"] != logger.RedactedPlaceholder {
		t.Fatalf("events[0].api_key not masked: %v", events[0])
	}
	if events[1] != "non-map-item" {
		t.Fatalf("events[1] altered: %v", events[1])
	}
	if events[2] != 42 {
		t.Fatalf("events[2] altered: %v", events[2])
	}
}
