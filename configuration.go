package logger

import (
	"fmt"
	"strings"
)

// ConfigureOption mutates the bootstrap state applied by Configure. Use the
// `With*` constructors below to build options; the option set is open for
// extension without breaking source compatibility.
type ConfigureOption func(*configureState)

type configureState struct {
	rootLevel          int
	hasRootLevel       bool
	sinks              []Sink
	hasSinks           bool
	perLoggerLevels    map[string]int
	resourceAttributes Attrs
	hasResource        bool
	redactionSuffixes  []string
	hasRedaction       bool
	redactionDisabled  bool // replace_defaults=true with empty extras
}

// WithRootLevel sets the default minimum severity threshold for the root
// logger. The level argument may be a string (case-insensitive name like
// "INFO" or "warn") or an integer in the [1, 24] range.
func WithRootLevel(level any) ConfigureOption {
	return func(s *configureState) {
		n, err := resolveLevel(level)
		if err != nil {
			// Configure surfaces this via panic — keep this in the option
			// constructor for early discovery; tests catch via recover.
			panic(err)
		}
		s.rootLevel = n
		s.hasRootLevel = true
	}
}

// WithSinks attaches the provided sinks to the root logger. Children
// inherit unless they declare their own sinks.
func WithSinks(sinks ...Sink) ConfigureOption {
	return func(s *configureState) {
		s.sinks = append([]Sink(nil), sinks...)
		s.hasSinks = true
	}
}

// WithPerLoggerLevels overrides min-severity for the listed logger names.
// Useful for silencing noisy upstreams (e.g., {"net/http": "WARN"}).
func WithPerLoggerLevels(levels map[string]any) ConfigureOption {
	return func(s *configureState) {
		out := make(map[string]int, len(levels))
		for name, lv := range levels {
			n, err := resolveLevel(lv)
			if err != nil {
				panic(fmt.Errorf("per-logger level for %q: %w", name, err))
			}
			out[name] = n
		}
		s.perLoggerLevels = out
	}
}

// WithResourceAttributes installs process/service-level attributes on the
// root logger Resource (per spec §4.2). Inherited by all loggers.
func WithResourceAttributes(attrs Attrs) ConfigureOption {
	return func(s *configureState) {
		s.resourceAttributes = attrs
		s.hasResource = true
	}
}

// WithRedactionConfig registers a Phase 1 redaction policy (spec §10.4).
// The configured suffix list applies to every emit through the root
// logger; child loggers inherit unless their own redaction is set later.
//
// Validation runs at option-construction time — invalid suffixes (empty,
// whitespace, non-lowercase-ASCII) panic so the misconfiguration surfaces
// at startup, never at the first emit. This matches the Configure
// philosophy already used by WithRootLevel / WithPerLoggerLevels.
//
// Default behaviour (no WithRedactionConfig call) keeps the 6-element
// DefaultSecretSuffixes set.
func WithRedactionConfig(cfg RedactionConfig) ConfigureOption {
	return func(s *configureState) {
		if err := ValidateRedactionConfig(cfg); err != nil {
			panic(err)
		}
		s.redactionSuffixes = BuildEffectiveSuffixes(cfg)
		s.hasRedaction = true
		s.redactionDisabled = cfg.ReplaceDefaults && len(cfg.ExtraSuffixes) == 0
	}
}

// WithAutoInjectTraceContext is the cross-binding parity flag declared in
// spec ADR-0001 v1.2 §3.4.2. The Go binding's idiomatic API surface is the
// explicit-ctx mode (Get(name).InfoCtx(ctx, ...) etc.) per §3.4.1; the
// auto-inject mode is **declared unsupported** in this binding to honour
// Go's no-implicit-context invariant.
//
// Calling this option with `enabled=false` is a no-op and is provided so
// cross-binding configurations (Python / TypeScript) that explicitly set
// the flag to `false` can keep the Go-side configure call symmetric.
//
// Calling with `enabled=true` panics at option-construction time per
// §3.4.1 — a binding without an ambient-context primitive MUST surface a
// configuration error rather than silently no-op. Use the *Ctx severity
// methods (InfoCtx / ErrorCtx / etc.) for explicit context propagation.
func WithAutoInjectTraceContext(enabled bool) ConfigureOption {
	if enabled {
		panic(fmt.Errorf(
			"WithAutoInjectTraceContext(true) is not supported by go.dagstack.dev/logger: " +
				"the Go binding declares auto-inject mode unsupported per logger-spec ADR-0001 §3.4.1. " +
				"Use the *Ctx severity variants (InfoCtx, ErrorCtx, ...) for explicit context propagation",
		))
	}
	return func(_ *configureState) {
		// no-op for cross-binding configure-call symmetry
	}
}

// Configure applies the bootstrap options to the global logger state.
//
// The root logger is updated atomically: min-severity, sinks, per-logger
// level overrides, and the Resource attribute set. Unspecified groups
// preserve their previous values when called more than once (so a partial
// reconfigure stays safe).
//
// Invalid severity strings are rejected at option-construction time —
// WithRootLevel and WithPerLoggerLevels panic if the string does not
// resolve to a canonical level (TRACE / DEBUG / INFO / WARN / ERROR /
// FATAL or a 1–24 integer). Callers typically resolve options once at
// startup, where a panic is acceptable (recover-on-startup is a tested
// pattern). Configure itself does not validate further once the options
// have been built.
func Configure(opts ...ConfigureOption) {
	// Atomicity invariant: every ConfigureOption closure mutates only the
	// local state struct — never the global registry. Validation panics
	// (WithRootLevel / WithPerLoggerLevels / WithRedactionConfig) raise
	// before the apply loop below, so a malformed Configure call leaves
	// the registry untouched. This must be preserved as new ConfigureOption
	// constructors are added: side-effect-free option closures are part of
	// the public Configure contract.
	state := &configureState{}
	for _, opt := range opts {
		opt(state)
	}
	// Apply loop: the order below is intentional. Downstream Set*
	// methods MUST NOT panic post-validation, otherwise this sequence
	// becomes non-atomic on partial failure.
	root := Get("")
	if state.hasRootLevel {
		root.SetMinSeverity(state.rootLevel)
	}
	if state.hasSinks {
		root.SetSinks(state.sinks)
	}
	if state.hasResource {
		if state.resourceAttributes != nil {
			root.SetResource(&Resource{Attributes: copyAttrs(state.resourceAttributes)})
		} else {
			root.SetResource(nil)
		}
	}
	for name, level := range state.perLoggerLevels {
		Get(name).SetMinSeverity(level)
	}
	if state.hasRedaction {
		root.SetRedactionSuffixes(state.redactionSuffixes)
		if state.redactionDisabled {
			Get("dagstack.logger.internal").Warn(
				"redaction disabled by Configure: replace_defaults=true with empty extra_suffixes; "+
					"all suffix-based masking is OFF (spec §10.4 disable-all warning)",
				nil,
			)
		}
	}
}

var severityNameToNumber = map[string]int{
	"TRACE":    int(SeverityTrace),
	"DEBUG":    int(SeverityDebug),
	"INFO":     int(SeverityInfo),
	"WARN":     int(SeverityWarn),
	"WARNING":  int(SeverityWarn),
	"ERROR":    int(SeverityError),
	"FATAL":    int(SeverityFatal),
	"CRITICAL": int(SeverityFatal),
}

func resolveLevel(level any) (int, error) {
	switch v := level.(type) {
	case int:
		if !IsValidSeverityNumber(v) {
			return 0, fmt.Errorf("severity_number %d not in [1, 24]", v)
		}
		return v, nil
	case Severity:
		if !IsValidSeverityNumber(int(v)) {
			return 0, fmt.Errorf("severity_number %d not in [1, 24]", int(v))
		}
		return int(v), nil
	case string:
		upper := strings.ToUpper(v)
		if n, ok := severityNameToNumber[upper]; ok {
			return n, nil
		}
		return 0, fmt.Errorf("unknown severity name %q; expected one of %s", v, knownSeverityNames())
	default:
		return 0, fmt.Errorf("level must be string or int, got %T", level)
	}
}

func knownSeverityNames() string {
	names := make([]string, 0, len(severityNameToNumber))
	for n := range severityNameToNumber {
		names = append(names, n)
	}
	// Stable order — sort for diagnostics. Keep imports minimal.
	for i := 1; i < len(names); i++ {
		j := i
		for j > 0 && names[j-1] > names[j] {
			names[j-1], names[j] = names[j], names[j-1]
			j--
		}
	}
	return strings.Join(names, ",")
}
