package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

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

	var writer io.Writer = os.Stdout
	switch strings.ToLower(output) {
	case "stderr":
		writer = os.Stderr
	case "stdout":
		writer = os.Stdout
	}

	if strings.ToLower(format) == "console" {
		writer = zerolog.ConsoleWriter{
			Out:        writer,
			TimeFormat: time.RFC3339,
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
