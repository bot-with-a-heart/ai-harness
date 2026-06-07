package logging

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Options struct {
	Level string
	JSON  bool
	Out   io.Writer
}

func Configure(opts Options) error {
	level, err := ParseLevel(opts.Level)
	if err != nil {
		return err
	}

	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	var writer io.Writer = out
	if !opts.JSON {
		writer = zerolog.ConsoleWriter{
			Out:        out,
			TimeFormat: time.RFC3339,
			NoColor:    true,
		}
	}

	zerolog.SetGlobalLevel(level)
	log.Logger = zerolog.New(writer).With().Timestamp().Logger()
	return nil
}

func ParseLevel(value string) (zerolog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "warn", "warning":
		return zerolog.WarnLevel, nil
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	case "disabled", "off", "none":
		return zerolog.Disabled, nil
	default:
		return zerolog.NoLevel, fmt.Errorf("invalid log level %q; expected debug, info, warn, error, or disabled", value)
	}
}
