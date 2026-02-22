package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestInitDebug(t *testing.T) {
	Init("debug", "json", "stdout")
	l := Get()
	if l.GetLevel() != zerolog.DebugLevel {
		t.Errorf("expected debug level")
	}
}

func TestInitInfo(t *testing.T) {
	Init("info", "json", "stdout")
	l := Get()
	if l.GetLevel() != zerolog.InfoLevel {
		t.Errorf("expected info level")
	}
}

func TestInitWarn(t *testing.T) {
	Init("warn", "json", "stdout")
	l := Get()
	if l.GetLevel() != zerolog.WarnLevel {
		t.Errorf("expected warn level")
	}
}

func TestInitError(t *testing.T) {
	Init("error", "json", "stdout")
	l := Get()
	if l.GetLevel() != zerolog.ErrorLevel {
		t.Errorf("expected error level")
	}
}

func TestInitUnknown(t *testing.T) {
	Init("unknown", "json", "stdout")
	l := Get()
	if l.GetLevel() != zerolog.InfoLevel {
		t.Errorf("expected info level as default")
	}
}

func TestInitConsole(t *testing.T) {
	Init("info", "console", "stdout")
	l := Get()
	if l == nil {
		t.Error("expected logger")
	}
}

func TestInitStderr(t *testing.T) {
	Init("info", "json", "stderr")
	l := Get()
	if l == nil {
		t.Error("expected logger")
	}
}

func TestInitStdout(t *testing.T) {
	Init("info", "json", "stdout")
	l := Get()
	if l == nil {
		t.Error("expected logger")
	}
}

func TestInitCaseInsensitive(t *testing.T) {
	Init("DEBUG", "JSON", "STDOUT")
	l := Get()
	if l.GetLevel() != zerolog.DebugLevel {
		t.Errorf("expected debug level")
	}
}

func TestGet(t *testing.T) {
	Init("info", "json", "stdout")
	l := Get()
	if l == nil {
		t.Error("expected logger")
	}
}

func TestDebug(t *testing.T) {
	Init("debug", "json", "stdout")
	e := Debug()
	if e == nil {
		t.Error("expected event")
	}
}

func TestInfo(t *testing.T) {
	Init("info", "json", "stdout")
	e := Info()
	if e == nil {
		t.Error("expected event")
	}
}

func TestWarn(t *testing.T) {
	Init("warn", "json", "stdout")
	e := Warn()
	if e == nil {
		t.Error("expected event")
	}
}

func TestError(t *testing.T) {
	Init("error", "json", "stdout")
	e := Error()
	if e == nil {
		t.Error("expected event")
	}
}

func TestFatal(t *testing.T) {
	Init("fatal", "json", "stdout")
	e := Fatal()
	if e == nil {
		t.Error("expected event")
	}
}

func TestWith(t *testing.T) {
	Init("info", "json", "stdout")
	c := With()
	_ = c
}

func TestDebugWritesOutput(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()

	Debug().Msg("test message")

	if !strings.Contains(buf.String(), "test message") {
		t.Error("expected log message in output")
	}
}

func TestInfoWritesOutput(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf).Level(zerolog.InfoLevel).With().Timestamp().Logger()

	Info().Msg("test message")

	if !strings.Contains(buf.String(), "test message") {
		t.Error("expected log message in output")
	}
}

func TestWarnWritesOutput(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf).Level(zerolog.WarnLevel).With().Timestamp().Logger()

	Warn().Msg("test message")

	if !strings.Contains(buf.String(), "test message") {
		t.Error("expected log message in output")
	}
}

func TestErrorWritesOutput(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf).Level(zerolog.ErrorLevel).With().Timestamp().Logger()

	Error().Msg("test message")

	if !strings.Contains(buf.String(), "test message") {
		t.Error("expected log message in output")
	}
}
