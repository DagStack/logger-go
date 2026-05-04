package logger_test

import (
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func TestConfigureResourceAttributesAttached(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithResourceAttributes(logger.Attrs{"service.name": "pilot-app"}),
	)
	logger.Get("x").Info("msg", nil)
	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	rec := records[0]
	if rec.Resource == nil {
		t.Fatalf("Resource is nil")
	}
	if rec.Resource.Attributes["service.name"] != "pilot-app" {
		t.Fatalf("service.name = %v", rec.Resource.Attributes["service.name"])
	}
}

func TestConfigureLevelResolvesString(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("warn"),
		logger.WithSinks(sink),
	)
	log := logger.Get("x")
	log.Info("below", nil)
	log.Error("above", nil)
	if len(sink.Records()) != 1 {
		t.Fatalf("got %d records, want 1", len(sink.Records()))
	}
}

func TestConfigureLevelResolvesNumeric(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel(17),
		logger.WithSinks(sink),
	)
	log := logger.Get("x")
	log.Warn("below", nil)
	log.Error("above", nil)
	if len(sink.Records()) != 1 {
		t.Fatalf("got %d records, want 1", len(sink.Records()))
	}
}

func TestConfigureUnknownLevelPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for unknown level")
		}
		s, ok := r.(error)
		if !ok {
			t.Fatalf("panic value not error: %T %v", r, r)
		}
		if !strings.Contains(s.Error(), "unknown severity") {
			t.Fatalf("error %q does not mention unknown severity", s)
		}
	}()
	logger.Configure(logger.WithRootLevel("BOGUS"))
}

func TestConfigureInvalidNumericLevelPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for invalid numeric level")
		}
	}()
	logger.Configure(logger.WithRootLevel(100))
}

func TestConfigurePerLoggerLevels(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithPerLoggerLevels(map[string]any{"noisy": "WARN"}),
	)
	logger.Get("noisy").Info("should be dropped", nil)
	logger.Get("quiet").Info("should pass", nil)
	bodies := []string{}
	for _, r := range sink.Records() {
		bodies = append(bodies, r.Body.(string))
	}
	if contains(bodies, "should be dropped") {
		t.Errorf("noisy.Info passed: %v", bodies)
	}
	if !contains(bodies, "should pass") {
		t.Errorf("quiet.Info dropped: %v", bodies)
	}
}

func TestConfigureWithNilResourceClearsResource(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithResourceAttributes(nil),
	)
	logger.Get("x").Info("msg", nil)
	rec := sink.Records()[0]
	if rec.Resource != nil {
		t.Fatalf("Resource = %v, want nil", rec.Resource)
	}
}

func TestConfigureRedaction_ExtraSuffixesAdditive(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithRedactionConfig(logger.RedactionConfig{
			ExtraSuffixes: []string{"_apikey"},
		}),
	)
	logger.Get("x").Info("event", logger.Attrs{
		"openai_api_key": "sk-base",  // base — masked
		"stripe_apikey":  "sk-extra", // extra — masked
		"user.id":        17,         // safe
	})
	rec := sink.Records()[0]
	if rec.Attributes["openai_api_key"] != "***" {
		t.Fatalf("base suffix not masked: %v", rec.Attributes["openai_api_key"])
	}
	if rec.Attributes["stripe_apikey"] != "***" {
		t.Fatalf("extra suffix not masked: %v", rec.Attributes["stripe_apikey"])
	}
	if rec.Attributes["user.id"] != 17 {
		t.Fatalf("safe key altered: %v", rec.Attributes["user.id"])
	}
}

func TestConfigureRedaction_ReplaceDefaults(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithRedactionConfig(logger.RedactionConfig{
			ExtraSuffixes:   []string{"_password"},
			ReplaceDefaults: true,
		}),
	)
	logger.Get("x").Info("event", logger.Attrs{
		"openai_api_key": "sk-base",       // base — NOT masked under replace
		"db_password":    "real-password", // extra — masked
	})
	rec := sink.Records()[0]
	if rec.Attributes["openai_api_key"] == "***" {
		t.Fatalf("replace mode should drop base set; got masked")
	}
	if rec.Attributes["db_password"] != "***" {
		t.Fatalf("extra suffix not masked under replace: %v", rec.Attributes["db_password"])
	}
}

func TestConfigureRedaction_ReplaceWithEmptyDisablesAll(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithRedactionConfig(logger.RedactionConfig{
			ReplaceDefaults: true,
		}),
	)
	logger.Get("x").Info("event", logger.Attrs{
		"openai_api_key": "sk-base",
		"db_password":    "real-password",
	})
	rec := sink.Records()[0]
	if rec.Attributes["openai_api_key"] == "***" || rec.Attributes["db_password"] == "***" {
		t.Fatalf("disable-all mode should pass values unchanged")
	}
	// The disable-all WARN on dagstack.logger.internal MUST NOT route to
	// the application sinks under the default Configure flow — internal
	// is a separate diagnostic channel per spec §7.4 and defaults to
	// stderr.
	for _, r := range sink.Records() {
		if strings.Contains(asString(r.Body), "disable-all") {
			t.Fatalf("internal WARN leaked to application sink: %v", r.Body)
		}
	}
}

func TestInternalLoggerDefaultsToOwnSink(t *testing.T) {
	logger.ResetRegistryForTests()
	prodSink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(prodSink),
	)
	internal := logger.Get("dagstack.logger.internal")
	sinks := internal.EffectiveSinks()
	if len(sinks) == 0 {
		t.Fatalf("dagstack.logger.internal has no effective sinks")
	}
	for _, s := range sinks {
		if s.ID() == prodSink.ID() {
			t.Fatalf("internal logger inherited application sink %q — should default to its own", s.ID())
		}
	}
	// Also: explicit operator override MUST work — the contract is a
	// default, not a hardcode.
	customSink := logger.NewInMemorySink(10, 1)
	internal.SetSinks([]logger.Sink{customSink})
	got := internal.EffectiveSinks()
	if len(got) != 1 || got[0].ID() != customSink.ID() {
		t.Fatalf("operator override of internal sink lost: %v", got)
	}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func TestWithAutoInjectTraceContext_FalseIsNoOp(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	// Should not panic — false is the Go default and acceptable as
	// cross-binding configure-call symmetry.
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithAutoInjectTraceContext(false),
	)
	logger.Get("x").Info("ping", nil)
	if len(sink.Records()) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sink.Records()))
	}
}

func TestWithAutoInjectTraceContext_TruePanicsWithSpecGuidance(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for WithAutoInjectTraceContext(true)")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic value not error: %T %v", r, r)
		}
		// Spec guidance MUST mention §3.4.1 and the *Ctx variants.
		msg := err.Error()
		if !strings.Contains(msg, "§3.4.1") {
			t.Fatalf("error message missing §3.4.1 reference: %q", msg)
		}
		if !strings.Contains(msg, "InfoCtx") {
			t.Fatalf("error message missing *Ctx variant guidance: %q", msg)
		}
	}()
	logger.WithAutoInjectTraceContext(true)
}

func TestConfigureRedaction_PanicsOnInvalidSuffix(t *testing.T) {
	logger.ResetRegistryForTests()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on invalid suffix")
		}
	}()
	logger.Configure(
		logger.WithRedactionConfig(logger.RedactionConfig{
			ExtraSuffixes: []string{"_APIKEY"}, // uppercase rejected
		}),
	)
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
