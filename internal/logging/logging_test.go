package logging

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]zerolog.Level{
		"debug":    zerolog.DebugLevel,
		"info":     zerolog.InfoLevel,
		"warn":     zerolog.WarnLevel,
		"warning":  zerolog.WarnLevel,
		"error":    zerolog.ErrorLevel,
		"disabled": zerolog.Disabled,
	}
	for input, want := range cases {
		got, err := ParseLevel(input)
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseLevel(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestConfigureJSONDebugLogging(t *testing.T) {
	var out bytes.Buffer
	if err := Configure(Options{Level: "debug", JSON: true, Out: &out}); err != nil {
		t.Fatalf("configure: %v", err)
	}

	log.Debug().Str("component", "test").Msg("hello")

	got := out.String()
	if !strings.Contains(got, `"level":"debug"`) || !strings.Contains(got, `"component":"test"`) {
		t.Fatalf("unexpected log output:\n%s", got)
	}
}

func TestConfigureRejectsInvalidLevel(t *testing.T) {
	if err := Configure(Options{Level: "chatty", Out: &bytes.Buffer{}}); err == nil {
		t.Fatal("configure succeeded with invalid level")
	}
}
