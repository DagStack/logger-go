package logger

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"golang.org/x/term"
)

// ConsoleMode selects the output style of ConsoleSink per spec ADR-0001 §7.2.
type ConsoleMode int

const (
	// ConsoleAuto selects pretty when the stream is a TTY, json otherwise.
	ConsoleAuto ConsoleMode = iota
	// ConsoleJSON forces canonical JSON-lines output.
	ConsoleJSON
	// ConsolePretty forces single-line colored output (ANSI escape codes).
	ConsolePretty
)

// String returns the textual mode name (used in Sink.ID).
func (m ConsoleMode) String() string {
	switch m {
	case ConsoleJSON:
		return "json"
	case ConsolePretty:
		return "pretty"
	default:
		return "auto"
	}
}

// ANSI escape codes used in pretty mode.
const (
	ansiReset = "\033[0m"
	ansiDim   = "\033[2m"
)

var ansiSeverityColor = map[string]string{
	SeverityTextTrace: "\033[2;37m",
	SeverityTextDebug: "\033[36m",
	SeverityTextInfo:  "\033[32m",
	SeverityTextWarn:  "\033[33m",
	SeverityTextError: "\033[31m",
	SeverityTextFatal: "\033[1;31m",
}

// ConsoleSink writes LogRecords to stdout/stderr as JSON-lines or pretty
// colored text. Per spec §7.2 it is a Phase 1 MVP sink; the default mode
// auto-detects TTY for the destination stream.
type ConsoleSink struct {
	mode        ConsoleMode
	stream      io.Writer
	minSeverity int

	mu     sync.Mutex
	closed bool

	// isTTY caches the TTY check evaluated lazily on first emit; ConsoleAuto
	// uses it to decide between pretty and json.
	isTTY    bool
	ttyKnown bool

	id string
}

// ttyAware describes streams that implement an os.File-like Fd() method;
// io.Writer alone does not expose the file descriptor.
type ttyAware interface {
	Fd() uintptr
}

// NewConsoleSink constructs a ConsoleSink writing to stream in the given
// mode. When stream is nil, os.Stderr is used.
//
// minSeverity sets the early-drop threshold: records with severity_number
// below this value are skipped (use 1 to accept everything).
func NewConsoleSink(mode ConsoleMode, stream io.Writer, minSeverity int) *ConsoleSink {
	if stream == nil {
		stream = os.Stderr
	}
	return &ConsoleSink{
		mode:        mode,
		stream:      stream,
		minSeverity: minSeverity,
		id:          "console:" + mode.String(),
	}
}

// ID returns the URI-style sink identifier ("console:json", "console:pretty",
// "console:auto").
func (c *ConsoleSink) ID() string { return c.id }

// SupportsSeverity reports whether severityNumber meets the configured
// minimum severity threshold.
func (c *ConsoleSink) SupportsSeverity(severityNumber int) bool {
	return severityNumber >= c.minSeverity
}

// Emit writes record to the underlying stream. Errors are absorbed; sink
// failures must not propagate to the caller of Logger.Info.
func (c *ConsoleSink) Emit(record *LogRecord) {
	if record == nil {
		return
	}
	if !c.SupportsSeverity(record.SeverityNumber) {
		return
	}
	line, err := c.format(record)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	_, _ = io.WriteString(c.stream, line+"\n")
	// Best-effort flush for stream wrappers (bufio.Writer, etc.); ignored
	// for non-flushable writers.
	if f, ok := c.stream.(interface{ Sync() error }); ok {
		_ = f.Sync()
	}
}

// Flush is a no-op for synchronous console writes; the stream is flushed
// after every Emit. timeoutSeconds is ignored.
func (c *ConsoleSink) Flush(_ float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if f, ok := c.stream.(interface{ Sync() error }); ok {
		return f.Sync()
	}
	return nil
}

// Close marks the sink as closed and flushes the stream once. Subsequent
// Emit calls become no-ops. Close does not close stdout/stderr — those are
// shared with the process — only the close marker is set.
func (c *ConsoleSink) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if f, ok := c.stream.(interface{ Sync() error }); ok {
		_ = f.Sync()
	}
	return nil
}

func (c *ConsoleSink) format(record *LogRecord) (string, error) {
	if c.effectiveMode() == ConsoleJSON {
		return ToDagstackJSONL(record)
	}
	return c.formatPretty(record), nil
}

func (c *ConsoleSink) effectiveMode() ConsoleMode {
	if c.mode != ConsoleAuto {
		return c.mode
	}
	if !c.ttyKnown {
		c.isTTY = isTTYStream(c.stream)
		c.ttyKnown = true
	}
	if c.isTTY {
		return ConsolePretty
	}
	return ConsoleJSON
}

// isTTYStream returns true when the stream looks like a terminal — used
// for ConsoleAuto. A Writer without Fd() (e.g., bytes.Buffer in tests)
// is not a TTY. Uses golang.org/x/term — the upstream go-team-maintained
// implementation that handles all platforms (Unix isatty, Windows
// console mode, Plan 9, etc.) without our previous approximation via
// os.Stat + ModeCharDevice (architect review C5).
func isTTYStream(stream io.Writer) bool {
	tw, ok := stream.(ttyAware)
	if !ok {
		return false
	}
	return isTerminalFd(int(tw.Fd()))
}

// isTerminalFd is a small indirection so tests can substitute the
// platform-level call without depending on golang.org/x/term in test
// code paths.
var isTerminalFd = func(fd int) bool {
	return term.IsTerminal(fd)
}

func (c *ConsoleSink) formatPretty(record *LogRecord) string {
	color, hasColor := ansiSeverityColor[record.SeverityText]
	reset := ""
	if hasColor {
		reset = ansiReset
	}
	timestamp := formatTimestamp(record.TimeUnixNano)
	name := "root"
	if record.InstrumentationScope != nil && record.InstrumentationScope.Name != "" {
		name = record.InstrumentationScope.Name
	}
	body := formatBody(record.Body)
	out := fmt.Sprintf("%s%s%s [%s%s%s] %s%s%s: %s",
		ansiDim, timestamp, ansiReset,
		color, record.SeverityText, reset,
		ansiDim, name, ansiReset,
		body,
	)
	if len(record.Attributes) > 0 {
		out += " " + ansiDim + "|" + ansiReset + " " + formatAttrs(record.Attributes)
	}
	return out
}

func formatTimestamp(nano int64) string {
	t := time.Unix(0, nano).UTC()
	return t.Format("2006-01-02T15:04:05.000000") + "Z"
}

func formatBody(body any) string {
	if s, ok := body.(string); ok {
		return s
	}
	out, err := CanonicalJSONMarshalString(body)
	if err != nil {
		return fmt.Sprintf("%v", body)
	}
	return out
}

func formatAttrs(attrs Attrs) string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%s=%s", k, formatScalar(attrs[k]))
	}
	return out
}

func formatScalar(value any) string {
	switch v := value.(type) {
	case string:
		// Quote when the string contains spaces or "=" to keep tokens
		// parseable by simple readers.
		for _, r := range v {
			if r == ' ' || r == '=' {
				return fmt.Sprintf("%q", v)
			}
		}
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}
