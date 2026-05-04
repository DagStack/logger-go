# dagstack-logger-go

Go binding for [`dagstack/logger-spec`](https://github.com/dagstack/logger-spec) — OTel-compatible structured logging with named loggers, context propagation, scoped sink overrides, and dagstack JSON-lines wire format.

**Status: v0.2.0 (Phase 1).** Phase 1 sinks (Console, File, InMemory) are implemented, plus the Phase 1 redaction-config public API. OTLPSink, processor chain, and self-metrics arrive in Phase 2.

## Installation

```bash
go get go.dagstack.dev/logger
```

(The Go vanity URL `go.dagstack.dev/logger` resolves to `github.com/dagstack/logger-go`.)

## Usage

```go
package main

import (
    "context"
    "errors"
    "os"

    "go.dagstack.dev/logger"
)

func main() {
    logger.Configure(
        logger.WithRootLevel("INFO"),
        logger.WithSinks(logger.NewConsoleSink(logger.ConsoleAuto, os.Stderr, 1)),
        logger.WithResourceAttributes(map[string]any{
            "service.name":    "pilot-app",
            "service.version": "0.1.0",
        }),
    )

    log := logger.Get("dagstack.rag.retriever", "1.4.2")
    log.Info("query received", logger.Attrs{"user.id": 42})

    if err := doWork(); err != nil {
        log.Exception(err, logger.Attrs{"request.id": "req-abc"})
    }

    log.Close()
}

func doWork() error {
    return errors.New("simulated failure")
}
```

### Context propagation

```go
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

log := logger.Get("dagstack.rag")
log.InfoCtx(ctx, "in span") // trace_id, span_id auto-injected from ctx
```

### Scoped sink overrides

```go
mem := logger.NewInMemorySink(100, 1)
scoped := log.WithSinks(mem)
scoped.Info("captured only here")

records := mem.Records()
// records[0].Body == "captured only here"
```

For lexically bounded scope:

```go
err := log.ScopeSinks(ctx, []logger.Sink{mem}, func(ctx context.Context) error {
    return runAgentPipeline(ctx)
})
```

## Design choices specific to Go

- **`context.Context` first.** `Logger.InfoCtx(ctx, ...)` reads OTel trace state from `ctx`; `Logger.Info(...)` is provided for migration but skips trace propagation.
- **PascalCase exports.** `Logger.Info`, `Logger.WithSinks`, `Configure(...)`. Functional-options pattern (`WithRootLevel`, `WithSinks`, `WithPerLoggerLevels`, `WithResourceAttributes`).
- **Errors as values.** `Logger.Flush(timeout)` returns `*FlushResult, error`; sink `Close()` returns `error`. No panics from the public API.
- **`go.opentelemetry.io/otel/trace`** for context propagation — no parallel context implementation.
- **Variadic-typed attrs.** Attributes are passed as `Attrs` (a `map[string]any` alias) plus optional positional `slog.Attr`-like helpers in v0.2.

## Roadmap

- **v0.1.0** — Phase 1 sinks (Console, File, InMemory), severity emits, child loggers, scoped overrides, OTel context propagation, dagstack JSON-lines wire format, default redaction.
- **v0.2.0** (current) — Phase 1 redaction-config public API (`RedactionConfig` + `WithRedactionConfig`), `Reset()`, `WithAutoInjectTraceContext` cross-binding parity flag, ConsoleSink TTY detection via `golang.org/x/term`, RFC 8785 UTF-16 key sort. See `CHANGELOG.md` for the full list.
- **v0.3.0** — processor chain (redaction extra patterns, samplers), `slog.Attr`-style variadic attrs, `LoggerCtx` helpers.
- **v0.4.0** — `OTLPSink` (HTTP+protobuf), self-metrics via OTel Metrics SDK, runtime reconfigure.

## Specification

Normative decisions live in [`dagstack/logger-spec`](https://github.com/dagstack/logger-spec) ADR-0001 v1.2.

## Local development

```bash
git clone git@github.com:dagstack/logger-go.git
cd logger-go
make test           # go test -race ./...
make vet            # go vet
make coverage       # coverage report
make check          # full validation pass
```

## Related

- [`dagstack/logger-spec`](https://github.com/dagstack/logger-spec) — specification (source of truth).
- [`dagstack/logger-python`](https://github.com/dagstack/logger-python) — reference Python implementation.
- [`dagstack/logger-docs`](https://github.com/dagstack/logger-docs) — documentation and guides ([logger.dagstack.dev](https://logger.dagstack.dev)).

## License

Apache-2.0 — see `LICENSE`.
