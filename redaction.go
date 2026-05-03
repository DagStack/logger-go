package logger

import (
	"fmt"
	"strings"
)

// RedactedPlaceholder is the string used to replace secret-suffix attribute
// values per spec ADR-0001 §10.1 — a literal "***".
const RedactedPlaceholder = "***"

// DefaultSecretSuffixes is the canonical set of suffix patterns matched
// case-insensitively against attribute keys. Per spec §10.1 / §10.4 v1.1
// this is an opinionated 6-element subset of
// `config-spec/_meta/secret_patterns.yaml`. The list is fixed at v1.1 to
// preserve API stability; richer matchers ship via the Phase 2 processor
// pipeline (§10.3).
//
// A key whose lowercased form ends with any of these suffixes is treated
// as secret and its value is replaced with RedactedPlaceholder before
// serialization.
var DefaultSecretSuffixes = []string{
	"_key",
	"_secret",
	"_token",
	"_password",
	"_passphrase",
	"_credentials",
}

// RedactionConfig is the public Phase 1 surface for tuning suffix-based
// redaction (spec ADR-0001 §10.4). Applications register a config via
// WithRedactionConfig at Configure time.
//
// The zero value (no extras, ReplaceDefaults=false) keeps the Phase 1
// baseline: the 6-element DefaultSecretSuffixes set is applied with no
// additions. Calls without WithRedactionConfig keep the same baseline.
type RedactionConfig struct {
	// ExtraSuffixes are additional secret suffixes registered by the
	// application. Each entry MUST be lowercase ASCII, contain no
	// whitespace, and be non-empty (validated at option-construction time).
	ExtraSuffixes []string

	// ReplaceDefaults, when true, swaps the base set for ExtraSuffixes
	// instead of unioning. With ReplaceDefaults=true and an empty
	// ExtraSuffixes list, all suffix-based redaction is disabled — the
	// binding emits a WARN diagnostic on dagstack.logger.internal in
	// that case (spec §10.4 disable-all warning).
	ReplaceDefaults bool
}

// BuildEffectiveSuffixes produces the post-Configure suffix list applied
// during emit. When ReplaceDefaults=false, the result is the union of
// DefaultSecretSuffixes and ExtraSuffixes (deduplicated, lowercased).
// When ReplaceDefaults=true, only ExtraSuffixes are returned (also
// deduplicated and lowercased). Returns a non-nil empty slice when the
// resulting set is empty (replace-all-off mode); callers MUST treat that
// as "no suffix-based masking" — never fall back to DefaultSecretSuffixes
// silently, otherwise replace mode loses its intent.
//
// BuildEffectiveSuffixes does NOT validate cfg.ExtraSuffixes — callers
// passing untrusted input MUST run ValidateRedactionConfig first, or use
// WithRedactionConfig which validates at option-construction time.
func BuildEffectiveSuffixes(cfg RedactionConfig) []string {
	seen := map[string]struct{}{}
	// Non-nil empty so the disable-all case (replace_defaults=true,
	// extra=[]) survives nil-checks downstream — RedactAttributes treats
	// nil as "use defaults" but a non-nil empty list as "no masking".
	out := make([]string, 0, len(DefaultSecretSuffixes)+len(cfg.ExtraSuffixes))
	add := func(s string) {
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if !cfg.ReplaceDefaults {
		for _, s := range DefaultSecretSuffixes {
			add(s)
		}
	}
	for _, s := range cfg.ExtraSuffixes {
		add(strings.ToLower(s))
	}
	return out
}

// ValidateRedactionConfig returns an error when cfg.ExtraSuffixes contains
// an entry that is empty, whitespace-bearing, or not lowercase ASCII. Per
// spec §10.4 the binding MUST reject these at Configure time.
func ValidateRedactionConfig(cfg RedactionConfig) error {
	for i, s := range cfg.ExtraSuffixes {
		if s == "" {
			return fmt.Errorf("redaction.extra_suffixes[%d] contains an empty string", i)
		}
		if strings.ContainsAny(s, " \t\n\r\v\f") {
			return fmt.Errorf("redaction.extra_suffixes[%d] contains whitespace: %q", i, s)
		}
		if s != strings.ToLower(s) {
			return fmt.Errorf("redaction.extra_suffixes[%d] must be lowercase ASCII: %q", i, s)
		}
		for _, r := range s {
			if r > 127 {
				return fmt.Errorf("redaction.extra_suffixes[%d] must be lowercase ASCII: %q", i, s)
			}
		}
	}
	return nil
}

// IsSecretKey reports whether key matches any of the suffix patterns,
// case-insensitively. The suffixes parameter allows callers to override
// the default set; pass nil to use DefaultSecretSuffixes.
func IsSecretKey(key string, suffixes []string) bool {
	if suffixes == nil {
		suffixes = DefaultSecretSuffixes
	}
	lowered := strings.ToLower(key)
	for _, s := range suffixes {
		if strings.HasSuffix(lowered, s) {
			return true
		}
	}
	return false
}

// redactValue is the recursion helper for RedactAttributes: given a value
// whose carrier key is not itself secret, it walks nested map[string]any
// and []any so that secret-suffix keys at deeper levels are still masked
// (spec §10.2).
func redactValue(value any, suffixes []string) any {
	if nested, ok := value.(map[string]any); ok {
		return RedactAttributes(nested, suffixes)
	}
	if list, ok := value.([]any); ok {
		out := make([]any, len(list))
		for i, item := range list {
			out[i] = redactValue(item, suffixes)
		}
		return out
	}
	return value
}

// RedactAttributes returns a new attribute map where values of keys matching
// the secret suffixes are replaced with RedactedPlaceholder. The redaction
// is recursive both for nested map[string]any values and for []any slices
// whose items are map[string]any (per spec §10.2). A secret key buried
// inside a list of maps (for example, an event-stream payload) is masked
// even though the list key itself is not secret.
//
// The original attrs map is not mutated. Pass nil for suffixes to use
// DefaultSecretSuffixes.
func RedactAttributes(attrs Attrs, suffixes []string) Attrs {
	if attrs == nil {
		return nil
	}
	if suffixes == nil {
		suffixes = DefaultSecretSuffixes
	}
	out := make(Attrs, len(attrs))
	for key, value := range attrs {
		if IsSecretKey(key, suffixes) {
			out[key] = RedactedPlaceholder
			continue
		}
		out[key] = redactValue(value, suffixes)
	}
	return out
}
