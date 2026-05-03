// Package logger is the Go binding for dagstack/logger-spec — OTel-compatible
// structured logging with named loggers, automatic context propagation,
// scoped sink overrides, and a dagstack JSON-lines wire format.
//
// Spec: https://github.com/dagstack/logger-spec (ADR-0001 v1.2).
// Reference implementation: https://github.com/dagstack/logger-python.
//
// Status: v0.2.0 (Phase 1). Phase 1 sinks (Console, File, InMemory) are
// implemented, plus the Phase 1 redaction-config public API. OTLPSink,
// processor chain, and self-metrics arrive in Phase 2.
//
// Typical usage:
//
//	logger.Configure(
//	    logger.WithRootLevel("INFO"),
//	    logger.WithSinks(logger.NewConsoleSink(logger.ConsoleAuto, os.Stderr, 1)),
//	    logger.WithResourceAttributes(map[string]any{
//	        "service.name": "pilot-app",
//	    }),
//	)
//	log := logger.Get("dagstack.rag.retriever", "1.4.2")
//	log.Info("query received", logger.Attrs{"user.id": 42})
//
// Context propagation reads OTel trace state from the supplied context.Context.
// Use the *Ctx variants of the severity methods to enable trace_id/span_id
// auto-injection:
//
//	log.InfoCtx(ctx, "in span", nil)
//
// Scoped sink overrides per spec §6:
//
//	mem := logger.NewInMemorySink(100, 1)
//	scoped := log.WithSinks(mem)
//	scoped.Info("captured only here", nil)
//
// For lexically bounded scope, use ScopeSinks (callback-style; the spec's
// "Go ctx + defer" idiom is also exposed via WithSinks + manual restoration):
//
//	err := log.ScopeSinks(ctx, []logger.Sink{mem}, func(ctx context.Context) error {
//	    return runAgentPipeline(ctx)
//	})
//
// Phase 1 does not support runtime watch — OnReconfigure registers a
// Subscription with Active=false, InactiveReason is set to a diagnostic
// string, and the callback never fires.
package logger
