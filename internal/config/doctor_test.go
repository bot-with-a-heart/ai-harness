package config

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorPassesWithReachableDependencies(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected LM Studio health path: %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}

	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	provider := cfg.Providers["lmstudio"]["desktop"]
	provider.BaseURL = "http://127.0.0.1:1234/v1"
	cfg.Providers["lmstudio"]["desktop"] = provider

	contents, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := Init(path, InitOptions{}); err != nil {
		t.Fatalf("init config: %v", err)
	}
	if err := overwriteFile(path, contents); err != nil {
		t.Fatalf("write config: %v", err)
	}

	report := Doctor(DoctorOptions{
		Path:    path,
		Timeout: 5 * time.Second,
		Client:  client,
		LookPath: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
	})
	if !report.Passed() {
		t.Fatalf("doctor did not pass: %+v", report.Checks)
	}
}

func overwriteFile(path string, contents []byte) error {
	return os.WriteFile(path, contents, 0o600)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
