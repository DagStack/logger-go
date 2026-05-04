// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/guides/configure.mdx` (Go TabItem).
//
// The guide includes a `BuildSinks` factory (Step 2) plus a `bootstrap`
// orchestration function (Step 3) plus a per-logger-overrides example
// (Step 4). The test mirrors them verbatim and exercises BuildSinks with
// a temp file path so the disk I/O is sandboxed.

package docs_examples_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

// ── Step 2. "Build sinks from the config" — BuildSinks factory ─────────────

// SinkSpec mirrors the snippet's struct.
//
// --- snippet start (Step 2 — types and helpers) ----------------------------
type SinkSpec struct {
	Type        string
	Mode        string
	Path        string
	MaxBytes    int64
	Keep        int
	MinSeverity string
}

var severityNames = map[string]int{
	"TRACE": int(logger.SeverityTrace),
	"DEBUG": int(logger.SeverityDebug),
	"INFO":  int(logger.SeverityInfo),
	"WARN":  int(logger.SeverityWarn),
	"ERROR": int(logger.SeverityError),
	"FATAL": int(logger.SeverityFatal),
}

func resolveSeverity(name string) int {
	if name == "" {
		return int(logger.SeverityInfo)
	}
	if n, ok := severityNames[strings.ToUpper(name)]; ok {
		return n
	}
	return int(logger.SeverityInfo)
}

func BuildSinks(specs []SinkSpec) ([]logger.Sink, error) {
	out := make([]logger.Sink, 0, len(specs))
	for _, s := range specs {
		switch s.Type {
		case "console":
			mode := logger.ConsoleAuto
			switch s.Mode {
			case "json":
				mode = logger.ConsoleJSON
			case "pretty":
				mode = logger.ConsolePretty
			}
			out = append(out, logger.NewConsoleSink(mode, nil, resolveSeverity(s.MinSeverity)))
		case "file":
			fs, err := logger.NewFileSink(s.Path, s.MaxBytes, s.Keep, resolveSeverity(s.MinSeverity))
			if err != nil {
				return nil, fmt.Errorf("file sink %q: %w", s.Path, err)
			}
			out = append(out, fs)
		default:
			return nil, fmt.Errorf("unsupported sink type: %q", s.Type)
		}
	}
	return out, nil
}

// --- snippet end (Step 2) -------------------------------------------------

func TestConfigure_BuildSinksFactory(t *testing.T) {
	dir := t.TempDir()
	specs := []SinkSpec{
		{Type: "console", Mode: "json", MinSeverity: "INFO"},
		{Type: "file", Path: filepath.Join(dir, "out.jsonl"), MaxBytes: 0, Keep: 0, MinSeverity: "DEBUG"},
	}

	sinks, err := BuildSinks(specs)
	if err != nil {
		t.Fatalf("BuildSinks: %v", err)
	}
	if len(sinks) != 2 {
		t.Fatalf("len(sinks) = %d, want 2", len(sinks))
	}
	if got := sinks[0].ID(); got != "console:json" {
		t.Errorf("sinks[0].ID() = %q, want console:json", got)
	}
	if !sinks[0].SupportsSeverity(int(logger.SeverityInfo)) {
		t.Error("sinks[0] should support INFO")
	}
	if sinks[0].SupportsSeverity(int(logger.SeverityDebug)) {
		t.Error("sinks[0] should NOT support DEBUG (floor=INFO)")
	}
	if !sinks[1].SupportsSeverity(int(logger.SeverityDebug)) {
		t.Error("sinks[1] should support DEBUG")
	}

	// Unknown sink type → error path.
	if _, err := BuildSinks([]SinkSpec{{Type: "unknown"}}); err == nil {
		t.Error("BuildSinks([unknown]) did not return error")
	}

	// Cleanup file sink so the temp dir releases on Windows.
	if closer, ok := sinks[1].(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

// ── Step 3. "Call configure() at startup" — bootstrap orchestration ────────

// loggingSection mimics the typed section produced by the application's
// config loader. It exists to make the `bootstrap` snippet self-contained.
type loggingSection struct {
	Level    string
	Loggers  map[string]string
	Resource logger.Attrs
	Sinks    []SinkSpec
}

type appConfig struct {
	Logging loggingSection
}

func loadAppConfig() *appConfig {
	return &appConfig{
		Logging: loggingSection{
			Level: "INFO",
			Loggers: map[string]string{
				"net/http":               "WARN",
				"order_service.checkout": "DEBUG",
			},
			Resource: logger.Attrs{
				"service.name":    "order-service",
				"service.version": "1.0.0",
			},
			Sinks: []SinkSpec{},
		},
	}
}

func TestConfigure_BootstrapOrchestration(t *testing.T) {
	cfg := loadAppConfig()
	capture := installCapture(t, "order_service.bootstrap")

	// Inject the test capture sink rather than using BuildSinks against a
	// disk file (we don't need rotation here). The shape of the orchestration
	// — Configure with WithRootLevel/WithSinks/WithPerLoggerLevels/
	// WithResourceAttributes — is exactly what the docs show.

	// --- snippet start (Step 3, adapted) ------------------------------
	// Build per-logger level map from the parsed config.
	perLogger := make(map[string]any, len(cfg.Logging.Loggers))
	for name, level := range cfg.Logging.Loggers {
		perLogger[name] = level
	}

	logger.Configure(
		logger.WithRootLevel(cfg.Logging.Level),
		logger.WithSinks(capture), // docs: BuildSinks(...) → variadic spread
		logger.WithPerLoggerLevels(perLogger),
		logger.WithResourceAttributes(cfg.Logging.Resource),
	)

	logger.Get("order_service.bootstrap").Info("logger configured", nil)
	// --- snippet end --------------------------------------------------

	records := capture.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	r := records[0]
	if r.Body != "logger configured" {
		t.Errorf("Body = %v, want 'logger configured'", r.Body)
	}
	if r.Resource == nil || r.Resource.Attributes["service.name"] != "order-service" {
		t.Errorf("Resource service.name not propagated: %v", r.Resource)
	}

	// Per-logger override should take effect: "order_service.checkout" runs
	// at DEBUG even though root is INFO.
	checkoutLog := logger.Get("order_service.checkout")
	if got, want := checkoutLog.EffectiveMinSeverity(), int(logger.SeverityDebug); got != want {
		t.Errorf("EffectiveMinSeverity for order_service.checkout = %d, want %d", got, want)
	}
	// And root remains INFO.
	if got, want := logger.Get("").EffectiveMinSeverity(), int(logger.SeverityInfo); got != want {
		t.Errorf("Root EffectiveMinSeverity = %d, want %d", got, want)
	}
}

// ── Step 4. "Per-logger overrides" — direct Configure call ────────────────

func TestConfigure_PerLoggerOverrides(t *testing.T) {
	// --- snippet start -----------------------------------------------
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(logger.NewConsoleSink(logger.ConsoleAuto, nil, 1)),
		logger.WithPerLoggerLevels(map[string]any{
			"net/http":               "WARN",
			"order_service.checkout": "DEBUG",
		}),
		logger.WithResourceAttributes(logger.Attrs{"service.name": "order-service"}),
	)
	// --- snippet end -------------------------------------------------

	if got, want := logger.Get("net/http").EffectiveMinSeverity(), int(logger.SeverityWarn); got != want {
		t.Errorf("net/http EffectiveMinSeverity = %d, want %d (WARN)", got, want)
	}
	if got, want := logger.Get("order_service.checkout").EffectiveMinSeverity(), int(logger.SeverityDebug); got != want {
		t.Errorf("order_service.checkout EffectiveMinSeverity = %d, want %d (DEBUG)", got, want)
	}
	if got, want := logger.Get("").EffectiveMinSeverity(), int(logger.SeverityInfo); got != want {
		t.Errorf("root EffectiveMinSeverity = %d, want %d", got, want)
	}

	// Restore so subsequent tests aren't affected by the ConsoleSink.
	logger.Get("").SetSinks(nil)
}
