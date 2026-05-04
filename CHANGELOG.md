# Changelog

All notable changes to `go.dagstack.dev/logger` are recorded in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versioning — [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-05-03

Cross-binding parity wave per [`dagstack/logger-spec` architect review
epic](https://git.goldix.org/dagstack/logger-spec/issues/2). Closes M1,
M2, M3, M4, M5, S3, S9, C5 findings. Public
API changes are purely additive — `0.2.0` is a safe drop-in upgrade from
`0.1.1`. The only observable behaviour shift is the internal diagnostic
channel (`dagstack.logger.internal`) now routing to its own stderr sink
instead of inheriting application sinks; see Changed below.

### Added

- **Phase 1 redaction-config public API** (`RedactionConfig` + `WithRedactionConfig`)
  per logger-spec ADR-0001 v1.1 §10.4 (M3). Applications can now register
  extra secret suffixes at bootstrap without waiting for the Phase 2
  processor pipeline:

  ```go
  logger.Configure(
      logger.WithRedactionConfig(logger.RedactionConfig{
          ExtraSuffixes:   []string{"_apikey", "_x_internal_token"},
          // ReplaceDefaults: true,  // optional — narrows the safety net
      }),
  )
  ```

  Validation runs at option-construction time (panic on empty / whitespace
  / non-lowercase-ASCII suffix). When `ReplaceDefaults=true` and
  `ExtraSuffixes=[]`, all suffix-based masking is OFF and a WARN is
  emitted on `dagstack.logger.internal` per spec §10.4.

- **`Logger.SetRedactionSuffixes` + `Logger.EffectiveSecretSuffixes`** —
  programmatic accessors mirroring the configure-time surface.
  `BuildEffectiveSuffixes` and `ValidateRedactionConfig` are also exported
  so applications can compose policies before passing them to `Configure`.

- **`WithAutoInjectTraceContext(bool)` — cross-binding parity flag (M2)**
  per logger-spec ADR-0001 v1.2 §3.4.2. The Go binding declares auto-inject
  mode unsupported per §3.4.1 (Go's no-implicit-context invariant); the
  option is provided for cross-binding configure-call symmetry. Calling
  with `false` is a no-op; calling with `true` panics with spec guidance
  pointing to the `*Ctx` severity variants. Use `Get(name).InfoCtx(ctx, ...)`
  for explicit context propagation.

- **`Reset()`** package-level function (M1) — clears the global registry,
  restoring root defaults. For test isolation and hot-reload bootstrap
  loops. SAFETY: invalidates every logger handle held elsewhere; production
  code MUST NOT call it.

### Changed

- `DefaultSecretSuffixes` is now formally aligned with the spec §10.4 v1.1
  baseline — an opinionated 6-element subset of
  `config-spec/_meta/secret_patterns.yaml`. The set is identical to 0.1.x
  in content; the binding now references the spec section explicitly. The
  list is fixed at v1.1 to preserve API stability; richer matchers ship via
  the Phase 2 processor pipeline (§10.3).
- **`dagstack.logger.internal` defaults to its own stderr `ConsoleSink(JSON, WARN)`**
  on first `Get` (per spec §7.4) — diagnostic warnings (sink failures,
  configure-time disable-all, etc.) no longer silently merge with
  application sinks. Operators may opt in to merging with explicit
  `Get("dagstack.logger.internal").SetSinks(...)`.
- **`ConsoleSink` TTY detection now uses `golang.org/x/term.IsTerminal`**
  (C5) instead of an `os.Stat + ModeCharDevice` approximation. Fixes false
  positives on macOS and Windows where character devices are not necessarily
  TTYs, and false negatives on Linux pty pairs. Adds a runtime dependency
  on `golang.org/x/term` (already a transitive dep through
  `go.opentelemetry.io/otel`, no new module graph).

### Documentation

- **`Logger.SetMinSeverity` / `SetSinks` / `SetResource` doc-comments** (M1)
  now warn explicitly that the method mutates the *shared* per-logger
  registry node — every concurrent caller observes the change. Cross-references
  `WithSinks` / `ScopeSinks` / `Child` for non-shared scoping.
- **`Sink.Flush(timeoutSeconds)` doc-comment** (M4) clarifies that the
  timeout is a Phase 1 hint accepted for forward compatibility but **not
  enforced**; Phase 2 `OTLPSink` MUST honour the deadline. Cross-references
  spec §7.1.
- **`FileSink` doc-comment** (M5) adds an explicit symlink-follow caveat —
  the `path` argument is opened verbatim via stdlib `os.OpenFile`, which
  follows symbolic links by default. Hosts MUST treat the value as trusted.

### Fixed

- **Canonical JSON key order — UTF-16 code-unit sort per RFC 8785 §3.2.3
  (S3)**. Previously `sort.Strings` produced UTF-8 byte order, which
  diverged from `logger-typescript` (native `Object.keys().sort()`,
  UTF-16) on keys containing characters above U+FFFF (emoji, Han
  ideographs ≥ U+10000, etc.). Cross-binding wire-byte parity is now
  guaranteed even for non-BMP attribute keys. ASCII-only keys are
  unaffected.
- **`InMemorySink.ID()` collision** (S9) — multiple `InMemorySink` values
  created in the same process now get distinct ids (per-instance counter
  suffix) instead of all sharing `"in-memory"`. This unblocks
  `SetSinks([]Sink{a, b})` configurations where the registry deduplicates
  by id.

### Cross-binding parity

This release brings `go.dagstack.dev/logger` to parity with `dagstack-logger`
0.2.0 and `@dagstack/logger` 0.2.0 across all M-/S-/C-level architect review
findings. Conformance fixtures `redaction_extra_suffixes.json` and
`trace_context_propagation.json` from logger-spec v1.2 are exercised by the
test suite.

## [0.1.1] - 2026-05-03

Architect-review patch. Two security-relevant findings on `0.1.0`:

### Fixed

- **Recursive redaction now walks `[]any` lists of maps** (`redaction.go`).
  Previously a secret key buried inside a list of maps
  (`{"events": [{"api_key": "..."}]}`) escaped masking — a redaction gap
  for structured payloads typical of webhook bodies and audit trails. Lists
  of primitives stay untouched; only `map[string]any` items are recursed.
  Covered by five new regression tests (list-of-maps walk, primitive-list
  passthrough, mixed-list, plus whole-value-wins for lists and maps under
  secret keys).
- **`FileSink` documentation** explicitly warns that `path` is resolved
  with `filepath.Abs` and opened as-is — the host MUST treat the value as
  trusted and never accept it from end-user input or a plugin manifest. The
  runtime does not validate path itself; the contract is now stated
  explicitly.
- `context.go` — `whitelist` → `allow-list` for the W3C Baggage filter
  (already on `main` as of v0.1.1).
- `docs_examples/concepts_context_test.go` — same `whitelist` → `allow-list`
  in the doc-mirror comment.

Both redaction and FileSink findings come from the cross-binding architect
review of logger-spec; tracking issue:
[logger-spec#2](https://git.goldix.org/dagstack/logger-spec/issues/2).

Code that uses only flat attributes sees no behaviour change — safe
drop-in upgrade from `0.1.0`.

## [0.1.0] - 2026-04-26

Initial Phase 1 release tracking logger-spec ADR-0001 v1.0.

### New

- **Logger** — named loggers with dot-hierarchy (`Logger.Get(name, version)`),
  child loggers via `Logger.Child(attrs)`.
- **Severity emits** — `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Fatal`,
  generic `Log(severityNumber, ...)`, `Exception(err, ...)`.
- **Severity** — typed integer constants matching the spec's 1-24 range,
  with a canonical 6-string `severity_text` mapping.
- **Phase 1 sinks** — `ConsoleSink` (auto/json/pretty modes), `FileSink`
  (with native rotation via standard library), `InMemorySink` (capacity-bounded
  ring buffer for tests).
- **Scoped overrides** — `WithSinks`, `AppendSinks`, `WithoutSinks`,
  `ScopeSinks(ctx, sinks, fn)` per spec §6.
- **Context propagation** — automatic `trace_id`/`span_id`/`trace_flags`
  injection from `context.Context` via OTel API; `InfoCtx` / `WarnCtx`
  / `ErrorCtx` etc. variants.
- **Default redaction** — suffix-based masking on `*_key`, `*_secret`,
  `*_token`, `*_password`, `*_passphrase`, `*_credentials` (case-insensitive,
  recursive on nested maps).
- **Wire format** — dagstack JSON-lines emitter with Canonical JSON sorting
  (per config-spec §9.1.1); `trace_id`/`span_id` rendered as lowercase hex
  strings, timestamps as integer nanoseconds.
- **Configure** — functional-options bootstrap: `WithRootLevel`, `WithSinks`,
  `WithPerLoggerLevels`, `WithResourceAttributes`.
- **Subscription** — placeholder handle; Phase 1 returns `Active=false`
  with `InactiveReason` populated.

### Out of scope (Phase 2+)

- `OTLPSink`, processor chain, samplers, self-metrics, runtime reconfigure.

[Unreleased]: https://github.com/dagstack/logger-go/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/dagstack/logger-go/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/dagstack/logger-go/releases/tag/v0.1.1
[0.1.0]: https://github.com/dagstack/logger-go/releases/tag/v0.1.0
