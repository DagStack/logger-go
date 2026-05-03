package logger

import "fmt"

// Severity is the OTel severity_number, an integer in the inclusive range
// [1, 24]. Per spec ADR-0001 §2:
//
//   - 1-4   bucket → severity_text "TRACE"
//   - 5-8   bucket → severity_text "DEBUG"
//   - 9-12  bucket → severity_text "INFO"   (default INFO=9)
//   - 13-16 bucket → severity_text "WARN"
//   - 17-20 bucket → severity_text "ERROR"
//   - 21-24 bucket → severity_text "FATAL"
//
// Backends filter by exact match on severity_text, so the canonical 6-string
// set never grows. Numeric granularity (TRACE2, INFO3, ...) is carried in
// severity_number; intermediate values go through Logger.Log.
type Severity int

// Baseline severity values matching the public API methods.
const (
	SeverityTrace Severity = 1
	SeverityDebug Severity = 5
	SeverityInfo  Severity = 9
	SeverityWarn  Severity = 13
	SeverityError Severity = 17
	SeverityFatal Severity = 21
)

// Canonical severity_text strings — exactly the 6 OTel-recommended values
// per spec §2. Backends filter by exact match.
const (
	SeverityTextTrace = "TRACE"
	SeverityTextDebug = "DEBUG"
	SeverityTextInfo  = "INFO"
	SeverityTextWarn  = "WARN"
	SeverityTextError = "ERROR"
	SeverityTextFatal = "FATAL"
)

// CanonicalSeverityTexts is the ordered set of the 6 canonical severity_text
// strings. Bindings must not emit any other value for severity_text.
var CanonicalSeverityTexts = [6]string{
	SeverityTextTrace,
	SeverityTextDebug,
	SeverityTextInfo,
	SeverityTextWarn,
	SeverityTextError,
	SeverityTextFatal,
}

const (
	minSeverityNumber = 1
	maxSeverityNumber = 24
)

// SeverityTextFor maps a severity_number in [1, 24] to its canonical
// severity_text string. Returns an error if severity_number is out of range.
func SeverityTextFor(severityNumber int) (string, error) {
	if severityNumber < minSeverityNumber || severityNumber > maxSeverityNumber {
		return "", fmt.Errorf("severity_number must be in [%d, %d], got %d",
			minSeverityNumber, maxSeverityNumber, severityNumber)
	}
	switch {
	case severityNumber <= 4:
		return SeverityTextTrace, nil
	case severityNumber <= 8:
		return SeverityTextDebug, nil
	case severityNumber <= 12:
		return SeverityTextInfo, nil
	case severityNumber <= 16:
		return SeverityTextWarn, nil
	case severityNumber <= 20:
		return SeverityTextError, nil
	default:
		return SeverityTextFatal, nil
	}
}

// IsValidSeverityNumber reports whether severityNumber is in the valid
// [1, 24] range.
func IsValidSeverityNumber(severityNumber int) bool {
	return severityNumber >= minSeverityNumber && severityNumber <= maxSeverityNumber
}

// IsCanonicalSeverityText reports whether text is one of the 6 canonical
// OTel-recommended strings.
func IsCanonicalSeverityText(text string) bool {
	for _, t := range CanonicalSeverityTexts {
		if t == text {
			return true
		}
	}
	return false
}
