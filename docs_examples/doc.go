// Package docs_examples contains automated tests for the Go snippets in
// dagstack-logger-docs.
//
// Every *_test.go file mirrors one docs page and reproduces the Go snippet
// verbatim between the `// --- snippet start ---` / `// --- snippet end ---`
// markers, then asserts the expectations expressed in the snippet's comments.
//
// Run: `go test ./docs_examples/...`.
//
// The package uses `package docs_examples_test` (external test package) so
// it consumes go.dagstack.dev/logger through the public surface, exactly as a
// downstream user would. This catches regressions in the public API even when
// internal tests still pass.
//
// Each test uses a unique logger name (typically `t.Name()` or a constant
// prefixed with the test function) to avoid sharing global registry state
// between tests. The internal `ResetRegistryForTests` helper is not visible
// to external test packages.
//
// When docs and binding drift (a snippet calls a non-existent method or
// returns a different value), the test leaves an NB comment plus an assertion
// against the real behaviour, and the drift is tracked as a separate issue.
package docs_examples
