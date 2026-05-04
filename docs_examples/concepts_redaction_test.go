// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/concepts/redaction.mdx` (Go TabItem).

package docs_examples_test

import (
	"testing"

	"go.dagstack.dev/logger"
)

// ── "Behaviour" — flat suffix-matching redaction ──────────────────────────

func TestRedaction_FlatSuffixMasking(t *testing.T) {
	capture := installCapture(t, "auth")

	// --- snippet start -----------------------------------------------
	log := logger.Get("auth")

	log.Info("user authenticated", logger.Attrs{
		"user.id":       42,
		"api_key":       "sk-very-secret-value", // → "***"
		"session_token": "ey...",                // → "***"
		"request.id":    "req-abc",
	})
	// Emitted record:
	// attributes = {
	//   "user.id": 42,
	//   "api_key": "***",
	//   "session_token": "***",
	//   "request.id": "req-abc",
	// }
	// --- snippet end -------------------------------------------------

	r := capture.Records()[0]
	if r.Attributes["user.id"] != 42 {
		t.Errorf("user.id = %v, want 42 (passthrough)", r.Attributes["user.id"])
	}
	if r.Attributes["api_key"] != logger.RedactedPlaceholder {
		t.Errorf("api_key = %v, want %q", r.Attributes["api_key"], logger.RedactedPlaceholder)
	}
	if r.Attributes["session_token"] != logger.RedactedPlaceholder {
		t.Errorf("session_token = %v, want %q", r.Attributes["session_token"], logger.RedactedPlaceholder)
	}
	if r.Attributes["request.id"] != "req-abc" {
		t.Errorf("request.id = %v, want req-abc (passthrough)", r.Attributes["request.id"])
	}
}

// ── "Nested attributes" — recursive masking ───────────────────────────────

func TestRedaction_NestedRecursive(t *testing.T) {
	capture := installCapture(t, "auth.nested")
	log := logger.Get("auth.nested")

	// --- snippet start -----------------------------------------------
	log.Info("config snapshot", logger.Attrs{
		"config": map[string]any{
			"service.name": "order-service",
			"auth": map[string]any{
				"client_secret": "shh", // → "***"
				"redirect_url":  "https://...",
			},
		},
	})
	// Result mirrors the Python / TS examples — recursion through nested
	// map[string]any walks every key and masks matching suffixes.
	// --- snippet end -------------------------------------------------

	r := capture.Records()[0]
	cfg, ok := r.Attributes["config"].(map[string]any)
	if !ok {
		t.Fatalf("attributes.config not a map: %T (%v)", r.Attributes["config"], r.Attributes["config"])
	}
	if cfg["service.name"] != "order-service" {
		t.Errorf("config.service.name = %v, want order-service", cfg["service.name"])
	}
	auth, ok := cfg["auth"].(map[string]any)
	if !ok {
		t.Fatalf("config.auth not a map: %T", cfg["auth"])
	}
	if auth["client_secret"] != logger.RedactedPlaceholder {
		t.Errorf("auth.client_secret = %v, want %q", auth["client_secret"], logger.RedactedPlaceholder)
	}
	if auth["redirect_url"] != "https://..." {
		t.Errorf("auth.redirect_url = %v, want passthrough", auth["redirect_url"])
	}
}
