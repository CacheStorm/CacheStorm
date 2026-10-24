package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

var log zerolog.Logger

func init() {
	// Default to a working logger so uninitialized usage doesn't panic
	log = zerolog.New(os.Stderr).Level(zerolog.InfoLevel).With().Timestamp().Logger()
}

func Init(level, format, output string) {
	var lvl zerolog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = zerolog.DebugLevel
	case "info":
		lvl = zerolog.InfoLevel
	case "warn":
		lvl = zerolog.WarnLevel
	case "error":
		lvl = zerolog.ErrorLevel
	default:
		lvl = zerolog.InfoLevel
	}

	out := os.Stdout
	switch strings.ToLower(output) {
	case "stderr":
		out = os.Stderr
	case "stdout":
		out = os.Stdout
	}

	var writer io.Writer = out
	if strings.EqualFold(format, "console") {
		// Colour codes only make sense on a terminal; in a container or a
		// log file they arrive as literal escape sequences.
		writer = zerolog.ConsoleWriter{
			Out:        out,
			TimeFormat: time.RFC3339,
			NoColor:    !isatty.IsTerminal(out.Fd()) && !isatty.IsCygwinTerminal(out.Fd()),
		}
	}

	log = zerolog.New(writer).Level(lvl).With().Timestamp().Logger()
}

func Get() *zerolog.Logger {
	return &log
}

func Debug() *zerolog.Event {
	return log.Debug()
}

func Info() *zerolog.Event {
	return log.Info()
}

func Warn() *zerolog.Event {
	return log.Warn()
}

func Error() *zerolog.Event {
	return log.Error()
}

func Fatal() *zerolog.Event {
	return log.Fatal()
}

func With() zerolog.Context {
	return log.With()
}

// RecoverPanic logs a recovered panic with context. Use as: defer logger.RecoverPanic("component")
func RecoverPanic(component string) {
	if r := recover(); r != nil {
		log.Error().
			Str("component", component).
			Interface("panic", r).
			Msg("panic recovered in goroutine")
	}
}
