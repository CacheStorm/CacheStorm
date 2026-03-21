package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// Cover RecoverPanic: when a panic actually occurs
func TestRecoverPanicWithPanic(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf).Level(zerolog.ErrorLevel).With().Timestamp().Logger()

	func() {
		defer RecoverPanic("test-component")
		panic("test panic value")
	}()

	output := buf.String()
	if !strings.Contains(output, "panic recovered in goroutine") {
		t.Error("expected panic recovery log message")
	}
	if !strings.Contains(output, "test-component") {
		t.Error("expected component name in log output")
	}
	if !strings.Contains(output, "test panic value") {
		t.Error("expected panic value in log output")
	}
}

// Cover RecoverPanic: when no panic occurs (r == nil path)
func TestRecoverPanicNoPanic(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf).Level(zerolog.ErrorLevel).With().Timestamp().Logger()

	func() {
		defer RecoverPanic("safe-component")
		// No panic here
	}()

	if buf.Len() != 0 {
		t.Error("expected no log output when no panic occurs")
	}
}

// Cover Init with unknown output (not stdout or stderr - falls through to default stdout)
func TestInitUnknownOutput(t *testing.T) {
	Init("info", "json", "somefile")
	l := Get()
	if l == nil {
		t.Error("expected logger")
	}
}

// Cover Init with console format on stderr
func TestInitConsoleStderr(t *testing.T) {
	Init("info", "console", "stderr")
	l := Get()
	if l == nil {
		t.Error("expected logger")
	}
}

// Cover RecoverPanic: panic with non-string value
func TestRecoverPanicWithIntValue(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf).Level(zerolog.ErrorLevel).With().Timestamp().Logger()

	func() {
		defer RecoverPanic("int-panic")
		panic(42)
	}()

	output := buf.String()
	if !strings.Contains(output, "panic recovered in goroutine") {
		t.Error("expected panic recovery log message")
	}
	if !strings.Contains(output, "42") {
		t.Error("expected panic value 42 in log output")
	}
}
