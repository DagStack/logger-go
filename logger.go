package logger

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Logger is the primary handle for emitting LogRecords. Per spec ADR-0001 §3
// it provides named loggers with dot-hierarchy, severity emits, child
// bindings, and scoped sink overrides.
//
// Construct via Get — direct construction is reserved for the binding
// internals. The hierarchy is the dot-prefix of the name:
//
//	"dagstack.rag.retriever" → parent "dagstack.rag" → "dagstack" → root ""
//
// Sinks and min-severity inherit from the parent unless overridden on the
// child via SetSinks / SetMinSeverity.
//
// Context propagation reads OTel trace state from a context.Context. Use the
// *Ctx variants (InfoCtx, ErrorCtx, ...) to enable trace_id/span_id
// auto-injection; the non-Ctx variants (Info, Error, ...) skip propagation.
type Logger struct {
	name    string
	version string
	parent  *Logger

	mu               sync.Mutex
	sinks            []Sink
	sinksExplicit    bool
	minSeverity      int
	minSetExplicit   bool
	attributes       Attrs
	resource         *Resource
	scope            *InstrumentationScope
	suffixes         []string
	suffixesExplicit bool // mirrors sinksExplicit; distinguishes "inherit" vs "explicit-empty / disable-all"
}

// Logger registry — the dot-name → instance map. RWMutex allows concurrent
// reads (Get on already-cached names) while still serializing creations.
var (
	registryMu sync.RWMutex
	registry   = map[string]*Logger{}
)

// Get returns the cached logger with the given name; if absent, creates one
// and links it into the parent chain. Pass an empty name to obtain the root
// logger; pass a non-empty version to associate it with the
// instrumentation_scope.
//
// Repeated Get calls with the same name return the same instance. To attach
// or update an instrumentation-scope version, use GetVersioned.
func Get(name string) *Logger {
	return getOrCreate(name, "")
}

// GetVersioned returns the cached logger for name, attaching or updating
// the supplied instrumentation-scope version on the singleton. Calling
// GetVersioned with the same name and a different version updates the
// existing logger's scope in place.
func GetVersioned(name, version string) *Logger {
	return getOrCreate(name, version)
}

// internalLoggerName is the diagnostic channel for binding-internal
// warnings (sink failures, configure-time disable-all, etc.) per spec
// §7.4. This logger MUST default to a stderr ConsoleSink so its output
// never silently merges with application sinks — operators may opt in
// to merging by calling SetSinks explicitly.
const internalLoggerName = "dagstack.logger.internal"

func getOrCreate(name, version string) *Logger {
	registryMu.RLock()
	if l, ok := registry[name]; ok {
		registryMu.RUnlock()
		if version != "" && l.version != version {
			l.mu.Lock()
			l.version = version
			l.scope = &InstrumentationScope{Name: scopeNameFor(name), Version: version}
			l.mu.Unlock()
		}
		return l
	}
	registryMu.RUnlock()

	registryMu.Lock()
	defer registryMu.Unlock()
	if l, ok := registry[name]; ok {
		// Lost the race; another goroutine created it.
		if version != "" && l.version != version {
			l.mu.Lock()
			l.version = version
			l.scope = &InstrumentationScope{Name: scopeNameFor(name), Version: version}
			l.mu.Unlock()
		}
		return l
	}

	var parent *Logger
	if name != "" {
		parentName := parentNameOf(name)
		parent = getUnlocked(parentName)
	}
	l := &Logger{
		name:        name,
		version:     version,
		parent:      parent,
		minSeverity: 1,
		attributes:  Attrs{},
		scope:       &InstrumentationScope{Name: scopeNameFor(name), Version: version},
		suffixes:    nil, // nil means inherit; root falls back to DefaultSecretSuffixes
	}
	if name == internalLoggerName {
		l.sinks = []Sink{NewConsoleSink(ConsoleJSON, nil, int(SeverityWarn))}
		l.sinksExplicit = true
	}
	registry[name] = l
	return l
}

// getUnlocked returns the cached logger, creating it (recursively) under the
// caller's already-held registryMu. Used internally to walk the parent chain
// without releasing the write lock.
func getUnlocked(name string) *Logger {
	if l, ok := registry[name]; ok {
		return l
	}
	var parent *Logger
	if name != "" {
		parent = getUnlocked(parentNameOf(name))
	}
	l := &Logger{
		name:        name,
		version:     "",
		parent:      parent,
		minSeverity: 1,
		attributes:  Attrs{},
		scope:       &InstrumentationScope{Name: scopeNameFor(name)},
		suffixes:    nil, // nil means inherit; root falls back to DefaultSecretSuffixes
	}
	if name == internalLoggerName {
		l.sinks = []Sink{NewConsoleSink(ConsoleJSON, nil, int(SeverityWarn))}
		l.sinksExplicit = true
	}
	registry[name] = l
	return l
}

// resetRegistry clears the global registry — test-only helper, exported via
// export_test.go.
func resetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]*Logger{}
}

// Name returns the logger's dot-notation name (empty for root).
func (l *Logger) Name() string { return l.name }

// Version returns the instrumentation_scope version, or empty when unset.
func (l *Logger) Version() string { return l.version }

// EffectiveSinks resolves the sink list — explicit on this logger or
// inherited from the parent chain.
func (l *Logger) EffectiveSinks() []Sink {
	l.mu.Lock()
	if l.sinksExplicit {
		out := make([]Sink, len(l.sinks))
		copy(out, l.sinks)
		l.mu.Unlock()
		return out
	}
	parent := l.parent
	l.mu.Unlock()
	if parent != nil {
		return parent.EffectiveSinks()
	}
	return nil
}

// EffectiveMinSeverity resolves the early-drop threshold — explicit on this
// logger or inherited.
func (l *Logger) EffectiveMinSeverity() int {
	l.mu.Lock()
	if l.minSetExplicit {
		out := l.minSeverity
		l.mu.Unlock()
		return out
	}
	parent := l.parent
	l.mu.Unlock()
	if parent != nil {
		return parent.EffectiveMinSeverity()
	}
	return 1
}

// EffectiveResource resolves the Resource — explicit or inherited.
func (l *Logger) EffectiveResource() *Resource {
	l.mu.Lock()
	if l.resource != nil {
		out := l.resource
		l.mu.Unlock()
		return out
	}
	parent := l.parent
	l.mu.Unlock()
	if parent != nil {
		return parent.EffectiveResource()
	}
	return nil
}

// Configuration mutators (SetSinks / SetMinSeverity / SetResource) per
// spec ADR-0001 §3.1.
//
// WARNING: these methods mutate the *shared* registry node for this
// logger name. Get(name) returns a singleton per name, so any other
// goroutine that holds the same handle — or that calls Get(name) again
// — observes the updated state immediately. This is intentional for
// bootstrap (Get("").SetMinSeverity(...) propagates through the whole
// tree via parent inheritance) but it breaks naive test isolation: a
// test that overrides sinks on a well-known name leaks the override
// into the next test.
//
// For test isolation, call Reset() between tests, or use the scoped
// variants (§6) — WithSinks / AppendSinks / ScopeSinks — which return
// a fresh handle with overrides instead of mutating the shared state.

// SetSinks installs an explicit sink list on this logger.
//
// Mutates the shared registry node (visible to all consumers of the
// same name). Children inherit unless they set their own sinks.
func (l *Logger) SetSinks(sinks []Sink) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sinks = append([]Sink(nil), sinks...)
	l.sinksExplicit = true
}

// SetMinSeverity sets the explicit early-drop threshold for this logger.
//
// Mutates the shared registry node (visible to all consumers of the
// same name). Children inherit unless they set their own threshold.
func (l *Logger) SetMinSeverity(severityNumber int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minSeverity = severityNumber
	l.minSetExplicit = true
}

// SetResource installs an explicit Resource on this logger.
//
// Mutates the shared registry node (visible to all consumers of the
// same name). Children inherit unless they set their own Resource.
func (l *Logger) SetResource(r *Resource) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.resource = r
}

// SetRedactionSuffixes installs the effective secret-suffix list on this
// logger (typically called on the root logger via Configure → spec §10.4).
//
// Mutates the shared registry node. The suffix list MUST already be
// validated and lowercased — use BuildEffectiveSuffixes to derive it from
// a RedactionConfig.
//
// Pass nil to fall back to inherited behaviour (parent's suffixes or
// DefaultSecretSuffixes at the root).
func (l *Logger) SetRedactionSuffixes(suffixes []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if suffixes == nil {
		l.suffixes = nil
		l.suffixesExplicit = false
		return
	}
	// Explicit override (any length, including empty for disable-all per
	// spec §10.4). The boolean flag carries the "is set" signal so we
	// don't have to reason about nil-vs-non-nil-empty anywhere else.
	out := make([]string, len(suffixes))
	copy(out, suffixes)
	l.suffixes = out
	l.suffixesExplicit = true
}

// EffectiveSecretSuffixes resolves the redaction-suffix list applied by
// this logger — explicit on this node or inherited from the parent
// chain. Returns DefaultSecretSuffixes when no override is registered
// anywhere up the chain. An explicit empty list (disable-all per spec
// §10.4) is preserved through inheritance — the returned slice is
// non-nil zero-length, distinguishable from "no override" via the
// suffixesExplicit flag on the resolving node.
//
// The returned slice is a snapshot copy; mutations do not affect the
// logger.
//
// TODO(#105): collapse the four chain-walks (sinks / min-severity /
// resource / suffixes) into one upward traversal — current impl
// acquires N locks per emit per dimension.
func (l *Logger) EffectiveSecretSuffixes() []string {
	l.mu.Lock()
	if l.suffixesExplicit {
		out := make([]string, len(l.suffixes))
		copy(out, l.suffixes)
		l.mu.Unlock()
		return out
	}
	parent := l.parent
	l.mu.Unlock()
	if parent != nil {
		return parent.EffectiveSecretSuffixes()
	}
	return append([]string(nil), DefaultSecretSuffixes...)
}

// Reset clears the global Logger registry — restores root defaults.
//
// For test isolation: call between tests that mutate logger state via
// SetSinks / SetMinSeverity / SetResource or via Configure. After
// Reset, a subsequent Get(name) creates a fresh node with no inherited
// overrides.
//
// SAFETY: this is a coarse instrument — it invalidates ALL logger
// handles still held elsewhere. Production code MUST NOT call this; it
// is reserved for test fixtures and the binding's own teardown.
func Reset() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]*Logger{}
}

// ─── Severity emits — non-context variants ────────────────────────────────

// Trace emits a TRACE-severity record (severity_number=1). attrs may be nil.
func (l *Logger) Trace(body any, attrs Attrs) { l.emit(nil, int(SeverityTrace), body, attrs) }

// Debug emits a DEBUG-severity record (severity_number=5). attrs may be nil.
func (l *Logger) Debug(body any, attrs Attrs) { l.emit(nil, int(SeverityDebug), body, attrs) }

// Info emits an INFO-severity record (severity_number=9). attrs may be nil.
func (l *Logger) Info(body any, attrs Attrs) { l.emit(nil, int(SeverityInfo), body, attrs) }

// Warn emits a WARN-severity record (severity_number=13). attrs may be nil.
func (l *Logger) Warn(body any, attrs Attrs) { l.emit(nil, int(SeverityWarn), body, attrs) }

// Error emits an ERROR-severity record (severity_number=17). attrs may be nil.
func (l *Logger) Error(body any, attrs Attrs) { l.emit(nil, int(SeverityError), body, attrs) }

// Fatal emits a FATAL-severity record (severity_number=21). attrs may be nil.
func (l *Logger) Fatal(body any, attrs Attrs) { l.emit(nil, int(SeverityFatal), body, attrs) }

// Log emits a record with the explicit severity_number (must be in [1, 24]).
// Use this for intermediate values like TRACE2 or INFO3 that share a
// severity_text bucket but a different numeric granularity.
func (l *Logger) Log(severityNumber int, body any, attrs Attrs) {
	l.emit(nil, severityNumber, body, attrs)
}

// Exception emits an ERROR-severity record with OTel exception.* attributes
// per spec §3.2 — exception.type, exception.message, exception.stacktrace.
//
// The stacktrace is captured via runtime/debug.Stack at the call site; for
// errors that wrap a stack via errors.New / fmt.Errorf the captured stack
// shows the logging call point (the most useful frame for triage).
//
// body may be nil — in that case err.Error() is used as the LogRecord body.
// Extra attrs override exception.* keys when supplied.
func (l *Logger) Exception(err error, body any, attrs Attrs) {
	l.exceptionWithCtx(nil, err, body, attrs)
}

// ─── Severity emits — context variants ────────────────────────────────────

// TraceCtx emits a TRACE record with trace_id/span_id auto-injected from ctx.
func (l *Logger) TraceCtx(ctx context.Context, body any, attrs Attrs) {
	l.emit(ctx, int(SeverityTrace), body, attrs)
}

// DebugCtx emits a DEBUG record with trace context auto-injected from ctx.
func (l *Logger) DebugCtx(ctx context.Context, body any, attrs Attrs) {
	l.emit(ctx, int(SeverityDebug), body, attrs)
}

// InfoCtx emits an INFO record with trace context auto-injected from ctx.
func (l *Logger) InfoCtx(ctx context.Context, body any, attrs Attrs) {
	l.emit(ctx, int(SeverityInfo), body, attrs)
}

// WarnCtx emits a WARN record with trace context auto-injected from ctx.
func (l *Logger) WarnCtx(ctx context.Context, body any, attrs Attrs) {
	l.emit(ctx, int(SeverityWarn), body, attrs)
}

// ErrorCtx emits an ERROR record with trace context auto-injected from ctx.
func (l *Logger) ErrorCtx(ctx context.Context, body any, attrs Attrs) {
	l.emit(ctx, int(SeverityError), body, attrs)
}

// FatalCtx emits a FATAL record with trace context auto-injected from ctx.
func (l *Logger) FatalCtx(ctx context.Context, body any, attrs Attrs) {
	l.emit(ctx, int(SeverityFatal), body, attrs)
}

// LogCtx is the generic emitter with explicit severity and context-aware
// trace propagation.
func (l *Logger) LogCtx(ctx context.Context, severityNumber int, body any, attrs Attrs) {
	l.emit(ctx, severityNumber, body, attrs)
}

// ExceptionCtx emits an ERROR record with OTel exception.* attributes plus
// trace context propagation from ctx.
func (l *Logger) ExceptionCtx(ctx context.Context, err error, body any, attrs Attrs) {
	l.exceptionWithCtx(ctx, err, body, attrs)
}

func (l *Logger) exceptionWithCtx(ctx context.Context, err error, body any, attrs Attrs) {
	if err == nil {
		return
	}
	excAttrs := Attrs{
		"exception.type":       fmt.Sprintf("%T", err),
		"exception.message":    err.Error(),
		"exception.stacktrace": string(debug.Stack()),
	}
	for k, v := range attrs {
		excAttrs[k] = v
	}
	finalBody := body
	if finalBody == nil {
		finalBody = err.Error()
	}
	l.emit(ctx, int(SeverityError), finalBody, excAttrs)
}

// ─── Scoped overrides (§6 spec) ──────────────────────────────────────────

// WithSinks returns a detached child logger whose sink list is replaced with
// the supplied set. The child is not cached in the global registry.
func (l *Logger) WithSinks(sinks ...Sink) *Logger {
	child := l.makeDetachedChild()
	child.sinks = append([]Sink(nil), sinks...)
	child.sinksExplicit = true
	return child
}

// AppendSinks returns a detached child logger whose sink list is the parent
// chain's effective sinks plus the supplied extras.
func (l *Logger) AppendSinks(extra ...Sink) *Logger {
	child := l.makeDetachedChild()
	base := l.EffectiveSinks()
	child.sinks = append(append([]Sink(nil), base...), extra...)
	child.sinksExplicit = true
	return child
}

// WithoutSinks returns a detached child logger with an empty sink list —
// emits go to /dev/null. Useful for silencing a sub-tree of operations.
func (l *Logger) WithoutSinks() *Logger {
	child := l.makeDetachedChild()
	child.sinks = nil
	child.sinksExplicit = true
	return child
}

// Child returns a detached child logger with the supplied attributes
// pre-bound to every record. Child-bound attrs are merged before call-site
// attrs, so call-site values win on collision.
func (l *Logger) Child(attrs Attrs) *Logger {
	child := l.makeDetachedChild()
	merged := make(Attrs, len(l.attributes)+len(attrs))
	for k, v := range l.attributes {
		merged[k] = v
	}
	for k, v := range attrs {
		merged[k] = v
	}
	child.attributes = merged
	return child
}

// ScopeSinks runs fn with a temporary sink override on this logger,
// restoring the previous sink set (and its explicit/inherit state) on
// return. Per spec §6.2 the Go idiom is the callback form — it pairs with
// context.Context and does not require defer at the call site.
//
// The override is applied to this logger instance directly, so any
// goroutines emitting through Logger.Get(name) during fn observe the same
// sinks. fn returns its error value; ScopeSinks does not modify it.
func (l *Logger) ScopeSinks(ctx context.Context, sinks []Sink, fn func(context.Context) error) error {
	l.mu.Lock()
	prevSinks := l.sinks
	prevExplicit := l.sinksExplicit
	l.sinks = append([]Sink(nil), sinks...)
	l.sinksExplicit = true
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.sinks = prevSinks
		l.sinksExplicit = prevExplicit
		l.mu.Unlock()
	}()

	if fn == nil {
		return nil
	}
	return fn(ctx)
}

func (l *Logger) makeDetachedChild() *Logger {
	child := &Logger{
		name:        l.name,
		version:     l.version,
		parent:      l,
		minSeverity: 1,
		attributes:  copyAttrs(l.attributes),
		scope:       l.scope,
		suffixes:    l.suffixes,
	}
	return child
}

// ─── Subscription (§7.2; Phase 1 inactive) ───────────────────────────────

// OnReconfigure registers a callback to fire when the logger's effective
// configuration changes. Phase 1 watch-based reconfigure is not implemented
// — the returned Subscription has Active=false and the callback never fires.
func (l *Logger) OnReconfigure(_ func()) *Subscription {
	return NewInactiveSubscription(
		"logger:"+l.name,
		"Phase 1 logger does not support watch-based reconfigure",
	)
}

// ─── Lifecycle ────────────────────────────────────────────────────────────

// FlushResult records the outcome of a Logger.Flush call. The returned
// shape mirrors the spec §13 contract; Phase 1 sinks always either succeed
// or fail with the underlying I/O error.
type FlushResult struct {
	// Success is true when every effective sink flushed without error.
	Success bool
	// Partial is true when at least one (but not all) sinks flushed
	// successfully.
	Partial bool
	// FailedSinks lists sinks that returned an error from Flush, with the
	// underlying error for diagnostics.
	FailedSinks []FlushFailure
}

// FlushFailure pairs a sink id with the error it returned from Flush.
type FlushFailure struct {
	SinkID string
	Err    error
}

// Flush attempts to flush every effective sink. timeoutSeconds is forwarded
// to each Sink.Flush; the global Flush itself does not enforce a deadline,
// so a cooperatively-implemented sink keeps the budget honest.
//
// Phase 1: every built-in sink (ConsoleSink, FileSink, InMemorySink) is
// synchronous, so timeoutSeconds is accepted for forward compatibility
// but is NOT enforced — none of the built-in sinks ever return a
// timeout error. Phase 2 (OTLPSink and friends) MUST honour the deadline
// and return a wrapped context.DeadlineExceeded; see spec ADR-0001 §7.1.
func (l *Logger) Flush(timeoutSeconds float64) (*FlushResult, error) {
	sinks := l.EffectiveSinks()
	result := &FlushResult{Success: true}
	if len(sinks) == 0 {
		return result, nil
	}
	successCount := 0
	for _, s := range sinks {
		if err := s.Flush(timeoutSeconds); err != nil {
			result.FailedSinks = append(result.FailedSinks, FlushFailure{
				SinkID: s.ID(),
				Err:    err,
			})
			continue
		}
		successCount++
	}
	if successCount == 0 {
		result.Success = false
	} else if successCount < len(sinks) {
		result.Success = false
		result.Partial = true
	}
	return result, nil
}

// Close calls Close on every effective sink. Errors are aggregated into the
// returned slice; Close on an already-closed sink is a no-op.
func (l *Logger) Close() error {
	sinks := l.EffectiveSinks()
	var firstErr error
	for _, s := range sinks {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ─── Internal emit ────────────────────────────────────────────────────────

func (l *Logger) emit(ctx context.Context, severityNumber int, body any, attrs Attrs) {
	if severityNumber < l.EffectiveMinSeverity() {
		return
	}
	severityText, err := SeverityTextFor(severityNumber)
	if err != nil {
		// Out-of-range severity is treated as INFO + a body marker; keeps
		// the call non-fatal but visible in logs.
		severityNumber = int(SeverityInfo)
		severityText = SeverityTextInfo
	}

	merged := Attrs{}
	for k, v := range l.attributes {
		merged[k] = v
	}
	for k, v := range attrs {
		merged[k] = v
	}

	merged = RedactAttributes(merged, l.EffectiveSecretSuffixes())

	var traceID, spanID []byte
	var traceFlags uint8
	if ctx != nil {
		traceID, spanID, traceFlags = ActiveTraceContext(ctx)
	}

	rec := &LogRecord{
		TimeUnixNano:         time.Now().UnixNano(),
		SeverityNumber:       severityNumber,
		SeverityText:         severityText,
		Body:                 body,
		Attributes:           merged,
		InstrumentationScope: l.scope,
		Resource:             l.EffectiveResource(),
		TraceID:              traceID,
		SpanID:               spanID,
		TraceFlags:           traceFlags,
	}

	for _, sink := range l.EffectiveSinks() {
		// Sink failure is isolated per spec §7.3 — a panic from one sink
		// must not propagate to the caller or break sibling sinks.
		func() {
			defer func() { _ = recover() }()
			sink.Emit(rec)
		}()
	}
}

func parentNameOf(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[:idx]
}

func scopeNameFor(name string) string {
	if name == "" {
		return "root"
	}
	return name
}
