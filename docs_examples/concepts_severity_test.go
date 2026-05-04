// Automated tests for the code snippets in
// `dagstack-logger-docs/site/docs/concepts/severity.mdx` (Go TabItem).

package docs_examples_test

import (
	"context"
	"testing"

	"go.dagstack.dev/logger"
)

// ── "Calling the severity methods" — six methods + InfoCtx ─────────────────

func TestSeverity_AllMethods(t *testing.T) {
	capture := installCapture(t, "order_service.checkout")

	ctx := context.Background()

	// --- snippet start -----------------------------------------------
	log := logger.Get("order_service.checkout")

	log.Trace("entering function", logger.Attrs{"args.order_id": 1234})
	log.Debug("cache miss", logger.Attrs{"cache.key": "user:42"})
	log.Info("order placed", logger.Attrs{"order.id": 1234})
	log.Warn("retry triggered", logger.Attrs{"retry.attempt": 2})
	log.Error("payment declined", logger.Attrs{"order.id": 1234})
	log.Fatal("config invariant violated", logger.Attrs{"reason": "missing service.name"})

	// Use the *Ctx variants when ctx carries an OTel span.
	log.InfoCtx(ctx, "order placed", logger.Attrs{"order.id": 1234})
	// --- snippet end -------------------------------------------------

	records := capture.Records()
	if len(records) != 7 {
		t.Fatalf("len(records) = %d, want 7", len(records))
	}

	wantSeverityTexts := []string{
		logger.SeverityTextTrace,
		logger.SeverityTextDebug,
		logger.SeverityTextInfo,
		logger.SeverityTextWarn,
		logger.SeverityTextError,
		logger.SeverityTextFatal,
		logger.SeverityTextInfo,
	}
	wantSeverityNumbers := []int{
		int(logger.SeverityTrace),
		int(logger.SeverityDebug),
		int(logger.SeverityInfo),
		int(logger.SeverityWarn),
		int(logger.SeverityError),
		int(logger.SeverityFatal),
		int(logger.SeverityInfo),
	}
	for i, r := range records {
		if r.SeverityText != wantSeverityTexts[i] {
			t.Errorf("records[%d].SeverityText = %q, want %q", i, r.SeverityText, wantSeverityTexts[i])
		}
		if r.SeverityNumber != wantSeverityNumbers[i] {
			t.Errorf("records[%d].SeverityNumber = %d, want %d", i, r.SeverityNumber, wantSeverityNumbers[i])
		}
	}
}

// ── "Intermediate severity" — log.LogCtx(ctx, 11, ...) ─────────────────────

func TestSeverity_LogCtxIntermediate(t *testing.T) {
	capture := installCapture(t, "severity.intermediate")

	ctx := context.Background()
	log := logger.Get("severity.intermediate")

	// --- snippet start -----------------------------------------------
	log.LogCtx(ctx, 11, "intermediate level", logger.Attrs{"phase": "warmup"})
	// severity_number=11 → severity_text="INFO" (still in 9-12 bucket).
	// --- snippet end -------------------------------------------------

	records := capture.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	r := records[0]
	if r.SeverityNumber != 11 {
		t.Errorf("SeverityNumber = %d, want 11", r.SeverityNumber)
	}
	if r.SeverityText != logger.SeverityTextInfo {
		t.Errorf("SeverityText = %q, want INFO (9-12 bucket)", r.SeverityText)
	}
	if r.Body != "intermediate level" {
		t.Errorf("Body = %v, want 'intermediate level'", r.Body)
	}
	if r.Attributes["phase"] != "warmup" {
		t.Errorf("phase = %v, want warmup", r.Attributes["phase"])
	}
}

// ── "The constants" — typed Severity int constants ────────────────────────

func TestSeverity_Constants(t *testing.T) {
	// --- snippet start -----------------------------------------------
	// import "go.dagstack.dev/logger"
	// (already imported above)

	// Typed Severity int constants.
	_ = logger.SeverityTrace // 1
	_ = logger.SeverityDebug // 5
	_ = logger.SeverityInfo  // 9
	_ = logger.SeverityWarn  // 13
	_ = logger.SeverityError // 17
	_ = logger.SeverityFatal // 21
	// --- snippet end -------------------------------------------------

	cases := []struct {
		name string
		got  logger.Severity
		want int
	}{
		{"TRACE", logger.SeverityTrace, 1},
		{"DEBUG", logger.SeverityDebug, 5},
		{"INFO", logger.SeverityInfo, 9},
		{"WARN", logger.SeverityWarn, 13},
		{"ERROR", logger.SeverityError, 17},
		{"FATAL", logger.SeverityFatal, 21},
	}
	for _, c := range cases {
		if int(c.got) != c.want {
			t.Errorf("Severity%s = %d, want %d", c.name, int(c.got), c.want)
		}
	}
}
