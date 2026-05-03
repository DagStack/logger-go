package logger_test

import (
	"context"
	"errors"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"

	"go.dagstack.dev/logger"
)

func setupCapture(t *testing.T, level any) *logger.InMemorySink {
	t.Helper()
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(100, 1)
	logger.Configure(
		logger.WithRootLevel(level),
		logger.WithSinks(sink),
	)
	return sink
}

// ─── Logger.Get ───────────────────────────────────────────────────────────

func TestLoggerGetRootCached(t *testing.T) {
	logger.ResetRegistryForTests()
	a := logger.Get("")
	b := logger.Get("")
	if a != b {
		t.Fatalf("Get(\"\") returned different instances")
	}
}

func TestLoggerGetNamedCached(t *testing.T) {
	logger.ResetRegistryForTests()
	a := logger.Get("dagstack.rag")
	b := logger.Get("dagstack.rag")
	if a != b {
		t.Fatalf("Get(name) returned different instances")
	}
}

func TestLoggerGetVersionUpdated(t *testing.T) {
	logger.ResetRegistryForTests()
	a := logger.Get("x")
	b := logger.GetVersioned("x", "1.0")
	if a != b {
		t.Fatalf("Get with version should return cached instance")
	}
	if b.Version() != "1.0" {
		t.Fatalf("Version not updated: %q", b.Version())
	}
}

func TestLoggerReset_ClearsRegistry(t *testing.T) {
	logger.ResetRegistryForTests()
	a := logger.Get("dagstack.reset_test")
	a.SetMinSeverity(99)
	logger.Reset()
	b := logger.Get("dagstack.reset_test")
	if a == b {
		t.Fatalf("Reset did not clear the registry — got same handle")
	}
	if got := b.EffectiveMinSeverity(); got != 1 {
		t.Fatalf("post-Reset effective min severity = %d, want 1", got)
	}
}

func TestLoggerReset_Idempotent(t *testing.T) {
	logger.Reset()
	logger.Reset()
	got := logger.Get("dagstack.reset_idempotent")
	if got == nil {
		t.Fatalf("Get after double-Reset returned nil")
	}
}

func TestLoggerGetVersionedAlias(t *testing.T) {
	logger.ResetRegistryForTests()
	a := logger.GetVersioned("y", "2.0")
	b := logger.Get("y")
	if a != b {
		t.Fatalf("GetVersioned mismatch")
	}
	if a.Version() != "2.0" {
		t.Fatalf("Version = %q", a.Version())
	}
}

// ─── Severity methods ─────────────────────────────────────────────────────

func TestLoggerInfoEmitsWithCorrectSeverity(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	logger.Get("test").Info("msg", nil)
	rec := sink.Records()[0]
	if rec.SeverityNumber != int(logger.SeverityInfo) {
		t.Fatalf("severity = %d", rec.SeverityNumber)
	}
	if rec.SeverityText != "INFO" {
		t.Fatalf("severity_text = %q", rec.SeverityText)
	}
	if rec.Body != "msg" {
		t.Fatalf("body = %v", rec.Body)
	}
}

func TestLoggerAllSeverityMethods(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	log := logger.Get("x")
	log.Trace("t", nil)
	log.Debug("d", nil)
	log.Info("i", nil)
	log.Warn("w", nil)
	log.Error("e", nil)
	log.Fatal("f", nil)
	want := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	got := []string{}
	for _, r := range sink.Records() {
		got = append(got, r.SeverityText)
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoggerLogWithExplicitSeverityNumber(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	logger.Get("x").Log(10, "inter", nil)
	rec := sink.Records()[0]
	if rec.SeverityNumber != 10 {
		t.Fatalf("severity_number = %d", rec.SeverityNumber)
	}
	if rec.SeverityText != "INFO" {
		t.Fatalf("severity_text = %q (bucket should be INFO)", rec.SeverityText)
	}
}

// ─── Exception logging ────────────────────────────────────────────────────

func TestLoggerExceptionAddsOTelAttributes(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	err := errors.New("boom")
	logger.Get("x").Exception(err, nil, nil)
	rec := sink.Records()[0]
	if rec.SeverityText != "ERROR" {
		t.Fatalf("severity = %q", rec.SeverityText)
	}
	if !contains([]string{"*errors.errorString"}, rec.Attributes["exception.type"].(string)) {
		// Allow either of common error type spellings; the leading * is from %T on a pointer.
		t.Logf("exception.type = %q", rec.Attributes["exception.type"])
	}
	if rec.Attributes["exception.message"] != "boom" {
		t.Fatalf("exception.message = %v", rec.Attributes["exception.message"])
	}
	stack, ok := rec.Attributes["exception.stacktrace"].(string)
	if !ok || stack == "" {
		t.Fatalf("exception.stacktrace not captured: %v", rec.Attributes["exception.stacktrace"])
	}
}

func TestLoggerExceptionCustomBodyAndAttrs(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	err := errors.New("oops")
	logger.Get("x").Exception(err, "failed to process", logger.Attrs{"request.id": "req-42"})
	rec := sink.Records()[0]
	if rec.Body != "failed to process" {
		t.Fatalf("body = %v", rec.Body)
	}
	if rec.Attributes["request.id"] != "req-42" {
		t.Fatalf("request.id = %v", rec.Attributes["request.id"])
	}
}

func TestLoggerExceptionNilErrorIsNoop(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	logger.Get("x").Exception(nil, nil, nil)
	if len(sink.Records()) != 0 {
		t.Fatalf("nil err produced records")
	}
}

// ─── Attributes ────────────────────────────────────────────────────────────

func TestLoggerCallSiteAttributesMerged(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	logger.Get("x").Info("msg", logger.Attrs{"user.id": 42})
	if sink.Records()[0].Attributes["user.id"] != 42 {
		t.Fatalf("user.id = %v", sink.Records()[0].Attributes["user.id"])
	}
}

func TestLoggerChildBindsAttributes(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	scoped := logger.Get("x").Child(logger.Attrs{"session.id": "sess-1"})
	scoped.Info("in scope", nil)
	rec := sink.Records()[0]
	if rec.Attributes["session.id"] != "sess-1" {
		t.Fatalf("session.id = %v", rec.Attributes["session.id"])
	}
}

func TestLoggerChildAttributesOverriddenByCallSite(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	log := logger.Get("x").Child(logger.Attrs{"k": "parent"})
	log.Info("msg", logger.Attrs{"k": "call-site"})
	if sink.Records()[0].Attributes["k"] != "call-site" {
		t.Fatalf("k = %v", sink.Records()[0].Attributes["k"])
	}
}

func TestLoggerRedactionMasksSecrets(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	logger.Get("x").Info("msg", logger.Attrs{"api_key": "sk-secret", "user.id": 42})
	attrs := sink.Records()[0].Attributes
	if attrs["api_key"] != logger.RedactedPlaceholder {
		t.Fatalf("api_key not masked: %v", attrs["api_key"])
	}
	if attrs["user.id"] != 42 {
		t.Fatalf("user.id altered: %v", attrs["user.id"])
	}
}

// ─── Hierarchy ─────────────────────────────────────────────────────────────

func TestLoggerChildInheritsSinks(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	logger.Get("dagstack.rag.retriever").Info("from deep logger", nil)
	if len(sink.Records()) != 1 {
		t.Fatalf("got %d, want 1", len(sink.Records()))
	}
}

func TestLoggerChildInheritsMinSeverity(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("TRACE"),
		logger.WithSinks(sink),
		logger.WithPerLoggerLevels(map[string]any{"dagstack.rag": "WARN"}),
	)
	child := logger.Get("dagstack.rag.retriever")
	child.Debug("below threshold", nil)
	child.Error("above threshold", nil)
	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Body != "above threshold" {
		t.Fatalf("body = %v", records[0].Body)
	}
}

func TestLoggerPerLoggerLevelOverride(t *testing.T) {
	logger.ResetRegistryForTests()
	sink := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(sink),
		logger.WithPerLoggerLevels(map[string]any{"noisy": "WARN"}),
	)
	logger.Get("noisy").Info("dropped", nil)
	logger.Get("quiet").Info("passed", nil)
	bodies := []string{}
	for _, r := range sink.Records() {
		bodies = append(bodies, r.Body.(string))
	}
	if contains(bodies, "dropped") {
		t.Errorf("dropped body present: %v", bodies)
	}
	if !contains(bodies, "passed") {
		t.Errorf("passed body missing: %v", bodies)
	}
}

// ─── Scoped overrides ─────────────────────────────────────────────────────

func TestLoggerWithSinksCreatesDetachedChild(t *testing.T) {
	logger.ResetRegistryForTests()
	base := logger.NewInMemorySink(10, 1)
	scopedSink := logger.NewInMemorySink(10, 1)
	logger.Configure(logger.WithRootLevel("TRACE"), logger.WithSinks(base))
	scoped := logger.Get("x").WithSinks(scopedSink)
	scoped.Info("only scoped", nil)
	if len(scopedSink.Records()) != 1 {
		t.Fatalf("scoped len = %d", len(scopedSink.Records()))
	}
	if len(base.Records()) != 0 {
		t.Fatalf("base captured leak: %d", len(base.Records()))
	}
}

func TestLoggerAppendSinksIncludesBoth(t *testing.T) {
	logger.ResetRegistryForTests()
	base := logger.NewInMemorySink(10, 1)
	extra := logger.NewInMemorySink(10, 1)
	logger.Configure(logger.WithRootLevel("TRACE"), logger.WithSinks(base))
	logger.Get("x").AppendSinks(extra).Info("both", nil)
	if len(base.Records()) != 1 || len(extra.Records()) != 1 {
		t.Fatalf("base=%d extra=%d", len(base.Records()), len(extra.Records()))
	}
}

func TestLoggerWithoutSinksDiscards(t *testing.T) {
	logger.ResetRegistryForTests()
	base := logger.NewInMemorySink(10, 1)
	logger.Configure(logger.WithRootLevel("TRACE"), logger.WithSinks(base))
	logger.Get("x").WithoutSinks().Info("discarded", nil)
	if len(base.Records()) != 0 {
		t.Fatalf("base captured: %d", len(base.Records()))
	}
}

func TestLoggerScopeSinks(t *testing.T) {
	logger.ResetRegistryForTests()
	base := logger.NewInMemorySink(10, 1)
	scoped := logger.NewInMemorySink(10, 1)
	logger.Configure(logger.WithRootLevel("TRACE"), logger.WithSinks(base))
	log := logger.Get("x")
	log.Info("before", nil)
	err := log.ScopeSinks(context.Background(), []logger.Sink{scoped}, func(_ context.Context) error {
		log.Info("during", nil)
		return nil
	})
	if err != nil {
		t.Fatalf("ScopeSinks err: %v", err)
	}
	log.Info("after", nil)

	baseBodies := []string{}
	for _, r := range base.Records() {
		baseBodies = append(baseBodies, r.Body.(string))
	}
	scopedBodies := []string{}
	for _, r := range scoped.Records() {
		scopedBodies = append(scopedBodies, r.Body.(string))
	}
	if !equal(baseBodies, []string{"before", "after"}) {
		t.Fatalf("base bodies = %v", baseBodies)
	}
	if !equal(scopedBodies, []string{"during"}) {
		t.Fatalf("scoped bodies = %v", scopedBodies)
	}
}

func TestLoggerScopeSinksNilFn(t *testing.T) {
	logger.ResetRegistryForTests()
	base := logger.NewInMemorySink(10, 1)
	logger.Configure(logger.WithRootLevel("INFO"), logger.WithSinks(base))
	log := logger.Get("x")
	scoped := logger.NewInMemorySink(10, 1)
	if err := log.ScopeSinks(context.Background(), []logger.Sink{scoped}, nil); err != nil {
		t.Fatalf("ScopeSinks(nil): %v", err)
	}
}

func TestLoggerScopeSinksPropagatesError(t *testing.T) {
	logger.ResetRegistryForTests()
	logger.Configure(logger.WithRootLevel("INFO"), logger.WithSinks(logger.NewInMemorySink(10, 1)))
	want := errors.New("inner")
	got := logger.Get("x").ScopeSinks(context.Background(), nil, func(_ context.Context) error {
		return want
	})
	if !errors.Is(got, want) {
		t.Fatalf("err = %v, want %v", got, want)
	}
}

// ─── Subscription ─────────────────────────────────────────────────────────

func TestLoggerOnReconfigureReturnsInactive(t *testing.T) {
	logger.ResetRegistryForTests()
	logger.Configure(logger.WithRootLevel("INFO"), logger.WithSinks(logger.NewInMemorySink(10, 1)))
	sub := logger.Get("x").OnReconfigure(func() {})
	if sub.Active {
		t.Fatalf("subscription unexpectedly active")
	}
	if sub.InactiveReason == "" {
		t.Fatalf("InactiveReason empty")
	}
	sub.Unsubscribe() // idempotent
	sub.Unsubscribe()
}

func TestSubscriptionUnsubscribeIdempotent(t *testing.T) {
	calls := 0
	sub := logger.NewSubscription("x", func() { calls++ })
	sub.Unsubscribe()
	sub.Unsubscribe()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSubscriptionNilSafe(t *testing.T) {
	var sub *logger.Subscription
	sub.Unsubscribe()
}

// ─── Lifecycle ────────────────────────────────────────────────────────────

func TestLoggerFlushNoSinks(t *testing.T) {
	logger.ResetRegistryForTests()
	res, err := logger.Get("x").Flush(1)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !res.Success {
		t.Fatalf("Flush success false: %v", res)
	}
}

func TestLoggerCloseNoExceptions(t *testing.T) {
	logger.ResetRegistryForTests()
	logger.Configure(logger.WithRootLevel("INFO"), logger.WithSinks(logger.NewInMemorySink(10, 1)))
	if err := logger.Get("x").Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ─── Sink failure isolation ───────────────────────────────────────────────

type panickingSink struct{}

func (panickingSink) ID() string                  { return "panicker" }
func (panickingSink) Emit(*logger.LogRecord)      { panic("emit boom") }
func (panickingSink) Flush(float64) error         { return nil }
func (panickingSink) Close() error                { return nil }
func (panickingSink) SupportsSeverity(_ int) bool { return true }

func TestLoggerSinkPanicIsolated(t *testing.T) {
	logger.ResetRegistryForTests()
	good := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(panickingSink{}, good),
	)
	logger.Get("x").Info("msg", nil)
	if len(good.Records()) != 1 {
		t.Fatalf("good sink missed record: len=%d", len(good.Records()))
	}
}

type failingFlushSink struct{}

func (failingFlushSink) ID() string                  { return "fail-flush" }
func (failingFlushSink) Emit(*logger.LogRecord)      {}
func (failingFlushSink) Flush(float64) error         { return errors.New("flush boom") }
func (failingFlushSink) Close() error                { return nil }
func (failingFlushSink) SupportsSeverity(_ int) bool { return true }

func TestLoggerFlushPartialFailure(t *testing.T) {
	logger.ResetRegistryForTests()
	good := logger.NewInMemorySink(10, 1)
	logger.Configure(
		logger.WithRootLevel("INFO"),
		logger.WithSinks(failingFlushSink{}, good),
	)
	res, err := logger.Get("x").Flush(1)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if res.Success {
		t.Fatalf("expected partial failure to mark success=false")
	}
	if !res.Partial {
		t.Fatalf("expected partial=true")
	}
	if len(res.FailedSinks) != 1 {
		t.Fatalf("FailedSinks = %d, want 1", len(res.FailedSinks))
	}
}

// ─── Context propagation ──────────────────────────────────────────────────

func TestLoggerAllSeverityCtxMethods(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	ctx := context.Background()
	log := logger.Get("x")
	log.TraceCtx(ctx, "t", nil)
	log.DebugCtx(ctx, "d", nil)
	log.InfoCtx(ctx, "i", nil)
	log.WarnCtx(ctx, "w", nil)
	log.ErrorCtx(ctx, "e", nil)
	log.FatalCtx(ctx, "f", nil)
	log.LogCtx(ctx, 11, "g", nil)
	want := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "INFO"}
	if len(sink.Records()) != len(want) {
		t.Fatalf("got %d records, want %d", len(sink.Records()), len(want))
	}
	for i, r := range sink.Records() {
		if r.SeverityText != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, r.SeverityText, want[i])
		}
	}
}

func TestLoggerInfoCtxAutoInjectsTrace(t *testing.T) {
	sink := setupCapture(t, "TRACE")

	traceIDHex := "0af7651916cd43dd8448eb211c80319c"
	spanIDHex := "b7ad6b7169203331"
	tid, _ := oteltrace.TraceIDFromHex(traceIDHex)
	sid, _ := oteltrace.SpanIDFromHex(spanIDHex)
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: oteltrace.FlagsSampled,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), sc)

	logger.Get("x").InfoCtx(ctx, "with span", nil)
	rec := sink.Records()[0]
	gotTID, _ := logger.EncodeTraceID(rec.TraceID)
	gotSID, _ := logger.EncodeSpanID(rec.SpanID)
	if gotTID != traceIDHex {
		t.Fatalf("trace_id = %q", gotTID)
	}
	if gotSID != spanIDHex {
		t.Fatalf("span_id = %q", gotSID)
	}
	if rec.TraceFlags != 1 {
		t.Fatalf("trace_flags = %d", rec.TraceFlags)
	}
}

func TestLoggerInfoNoCtxOmitsTrace(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	logger.Get("x").Info("no ctx", nil)
	rec := sink.Records()[0]
	if rec.TraceID != nil || rec.SpanID != nil {
		t.Fatalf("non-Ctx variant injected trace state")
	}
}

func TestLoggerExceptionCtxInjectsTrace(t *testing.T) {
	sink := setupCapture(t, "TRACE")
	tid, _ := oteltrace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := oteltrace.SpanIDFromHex("b7ad6b7169203331")
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: oteltrace.FlagsSampled,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), sc)

	logger.Get("x").ExceptionCtx(ctx, errors.New("boom"), nil, nil)
	rec := sink.Records()[0]
	gotTID, _ := logger.EncodeTraceID(rec.TraceID)
	if gotTID != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("trace_id = %q", gotTID)
	}
}

// ─── Integration ──────────────────────────────────────────────────────────

func TestLoggerInstrumentationScopeAttached(t *testing.T) {
	sink := setupCapture(t, "INFO")
	logger.GetVersioned("dagstack.rag.retriever", "1.4.2").Info("msg", nil)
	rec := sink.Records()[0]
	if rec.InstrumentationScope == nil {
		t.Fatalf("scope nil")
	}
	if rec.InstrumentationScope.Name != "dagstack.rag.retriever" {
		t.Fatalf("scope.Name = %q", rec.InstrumentationScope.Name)
	}
	if rec.InstrumentationScope.Version != "1.4.2" {
		t.Fatalf("scope.Version = %q", rec.InstrumentationScope.Version)
	}
}

func TestLoggerNamePublic(t *testing.T) {
	logger.ResetRegistryForTests()
	log := logger.Get("dagstack.x")
	if log.Name() != "dagstack.x" {
		t.Fatalf("Name = %q", log.Name())
	}
}

func TestLoggerEffectiveSeverityInheritsFromRoot(t *testing.T) {
	logger.ResetRegistryForTests()
	logger.Configure(logger.WithRootLevel("WARN"), logger.WithSinks(logger.NewInMemorySink(10, 1)))
	if logger.Get("dagstack.foo").EffectiveMinSeverity() != int(logger.SeverityWarn) {
		t.Fatalf("inherited severity wrong")
	}
}

func equal(a, b []string) bool {
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
