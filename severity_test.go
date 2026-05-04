package logger_test

import (
	"strings"
	"testing"

	"go.dagstack.dev/logger"
)

func TestSeverityBaselineValues(t *testing.T) {
	cases := []struct {
		name string
		sev  logger.Severity
		want int
	}{
		{"trace", logger.SeverityTrace, 1},
		{"debug", logger.SeverityDebug, 5},
		{"info", logger.SeverityInfo, 9},
		{"warn", logger.SeverityWarn, 13},
		{"error", logger.SeverityError, 17},
		{"fatal", logger.SeverityFatal, 21},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if int(tc.sev) != tc.want {
				t.Fatalf("Severity %s = %d, want %d", tc.name, int(tc.sev), tc.want)
			}
		})
	}
}

func TestCanonicalSeverityTextsExactSet(t *testing.T) {
	got := logger.CanonicalSeverityTexts
	want := [6]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	if got != want {
		t.Fatalf("CanonicalSeverityTexts = %v, want %v", got, want)
	}
}

func TestSeverityTextFor(t *testing.T) {
	cases := []struct {
		number int
		want   string
	}{
		{1, "TRACE"},
		{2, "TRACE"},
		{4, "TRACE"},
		{5, "DEBUG"},
		{8, "DEBUG"},
		{9, "INFO"},
		{12, "INFO"},
		{13, "WARN"},
		{16, "WARN"},
		{17, "ERROR"},
		{20, "ERROR"},
		{21, "FATAL"},
		{24, "FATAL"},
	}
	for _, tc := range cases {
		got, err := logger.SeverityTextFor(tc.number)
		if err != nil {
			t.Fatalf("SeverityTextFor(%d) error: %v", tc.number, err)
		}
		if got != tc.want {
			t.Fatalf("SeverityTextFor(%d) = %q, want %q", tc.number, got, tc.want)
		}
	}
}

func TestSeverityTextForOutOfRange(t *testing.T) {
	for _, n := range []int{0, -1, 25, 100} {
		_, err := logger.SeverityTextFor(n)
		if err == nil {
			t.Errorf("SeverityTextFor(%d) expected error", n)
			continue
		}
		if !strings.Contains(err.Error(), "[1, 24]") {
			t.Errorf("SeverityTextFor(%d) error %q does not mention [1, 24]", n, err)
		}
	}
}

func TestIsValidSeverityNumber(t *testing.T) {
	for _, n := range []int{1, 9, 24} {
		if !logger.IsValidSeverityNumber(n) {
			t.Errorf("IsValidSeverityNumber(%d) = false, want true", n)
		}
	}
	for _, n := range []int{0, -1, 25} {
		if logger.IsValidSeverityNumber(n) {
			t.Errorf("IsValidSeverityNumber(%d) = true, want false", n)
		}
	}
}

func TestIsCanonicalSeverityText(t *testing.T) {
	for _, text := range logger.CanonicalSeverityTexts {
		if !logger.IsCanonicalSeverityText(text) {
			t.Errorf("IsCanonicalSeverityText(%q) = false, want true", text)
		}
	}
	for _, bad := range []string{"trace", "info2", "Warning", "FATAL2", "CRITICAL", ""} {
		if logger.IsCanonicalSeverityText(bad) {
			t.Errorf("IsCanonicalSeverityText(%q) = true, want false", bad)
		}
	}
}
