# docs_examples

Automated tests for the Go snippets in [`dagstack-logger-docs`](https://github.com/dagstack/logger-docs).

Each `*_test.go` file mirrors one MDX page from `site/docs/` and reproduces the Go `<TabItem value="go">` snippet **verbatim** between the markers:

```go
// --- snippet start ---
... copied directly from the docs page ...
// --- snippet end ---
```

Surrounding scaffolding (`testing.T`, fixture data, deterministic stubs) makes the snippet executable; assertions verify it behaves as the docs prose claims.

## Coverage map

| MDX page | Test file |
|---|---|
| `intro.mdx` | `intro_test.go` |
| `concepts/severity.mdx` | `concepts_severity_test.go` |
| `concepts/sinks.mdx` | `concepts_sinks_test.go` |
| `concepts/context.mdx` | `concepts_context_test.go` |
| `concepts/operations.mdx` | `concepts_operations_test.go` |
| `concepts/redaction.mdx` | `concepts_redaction_test.go` |
| `concepts/scoped-overrides.mdx` | `concepts_scoped_overrides_test.go` |
| `guides/configure.mdx` | `guides_configure_test.go` |
| `guides/testing.mdx` | `guides_testing_test.go` |
| `guides/custom-sink.mdx` | `guides_custom_sink_test.go` |

`concepts/wire-formats.mdx` has no Go `<TabItem>` and is intentionally not mirrored. `concepts/ai-agents.mdx` is similarly skipped — Phase 1 has no agent-pack helpers in the binding.

## Run

```bash
go test -race ./docs_examples/...
```

Or via the repo Makefile (top-level), `make test`.

## Conventions

- **Package name** — `docs_examples_test` (external test package). The tests reach the binding only through its public surface, exactly as a downstream user would.
- **No internal helpers** — `logger.ResetRegistryForTests` is exposed via `export_test.go` for the in-package tests; it is **not** visible here. Tests isolate state by:
  - using a unique logger name (typically derived from `t.Name()` or a constant prefixed with the test function),
  - calling `logger.Configure(...)` with a fresh `InMemorySink` at the start of each test, and
  - cleaning up via `t.Cleanup` or a final `SetSinks(nil)` when the test mutates the root logger.
- **Verbatim snippets** — code between `// --- snippet start ---` and `// --- snippet end ---` is a literal copy of the MDX page. Do not refactor for "clean code". If a snippet doesn't compile or behaves differently from the prose, leave the snippet unchanged and add an `NB:` comment plus an assertion against the real behaviour; track the drift as a separate docs/binding fix.
- **Deterministic substitutes** — when a snippet references an external dependency the binding doesn't pull in (`github.com/google/uuid`, `github.com/getsentry/sentry-go`), tests replace the call with a fixed-value stub and document the substitution above the snippet block. The shape and surface of the substituted call match the docs.

## Adding a new test

1. Pick the MDX page you're covering.
2. Create `<page>_test.go` (slash → underscore in the path).
3. Copy the Go `<TabItem>` snippet verbatim into a `Test...` function, surrounded by `// --- snippet start ---` / `// --- snippet end ---`.
4. Add the minimum scaffolding (logger reset, InMemorySink, stub functions for `placeOrder`, `runBusinessLogic`, etc.) outside the snippet markers.
5. Assert the behaviour the surrounding MDX prose claims.
6. Run `go test -race ./docs_examples/...`. Verify no flakes under `-count=10`.

## Why this exists

The docs and the binding can drift independently — a snippet may call a method whose name has changed, a test fixture in MDX may quietly diverge from the binding's actual behaviour. Until a snippet is wired into a real `go test` run, that drift surfaces only when a user copy-pastes from the website and gets a compile error.

These tests run in the binding's CI, so docs-affecting binding changes break the build before they merge.
