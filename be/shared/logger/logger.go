package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New creates a zerolog.Logger bound to the given service.
//
// In development (env == "development") it writes human-readable output to
// stderr. In any other environment it writes structured JSON — ready for
// log aggregation tools like CloudWatch, Datadog, or Loki.
//
// All log lines will carry a "service" field, which is critical for
// distinguishing sources once multiple services write to the same aggregator.
func New(serviceName, level, env string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var out io.Writer
	if env == "development" {
		out = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05",
		}
	} else {
		out = os.Stderr
	}

	return zerolog.New(out).
		Level(lvl).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger()
}
