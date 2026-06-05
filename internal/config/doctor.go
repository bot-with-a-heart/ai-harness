package config

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type CheckStatus string

const (
	CheckPass CheckStatus = "PASS"
	CheckFail CheckStatus = "FAIL"
)

type Check struct {
	Name    string
	Status  CheckStatus
	Message string
	Err     error
}

type DoctorReport struct {
	Checks []Check
}

type DoctorOptions struct {
	Path     string
	Timeout  time.Duration
	Client   *http.Client
	LookPath func(string) (string, error)
}

func (r DoctorReport) Passed() bool {
	for _, check := range r.Checks {
		if check.Status != CheckPass {
			return false
		}
	}

	return true
}

func Doctor(opts DoctorOptions) DoctorReport {
	path := opts.Path
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return DoctorReport{Checks: []Check{fail("config path", "could not resolve default config path", err)}}
		}
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = lookPathPortable
	}

	var checks []Check
	if _, err := os.Stat(path); err != nil {
		checks = append(checks, fail("config exists", path, err))
		return DoctorReport{Checks: checks}
	}
	checks = append(checks, pass("config exists", path))

	cfg, err := Load(path)
	if err != nil {
		checks = append(checks, fail("config valid", path, err))
		return DoctorReport{Checks: checks}
	}
	checks = append(checks, pass("config valid", path))

	checks = append(checks, checkLMStudio(cfg, client, timeout))
	checks = append(checks, checkCodex(lookPath))
	checks = append(checks, checkWritePermissions(filepath.Dir(path)))
	checks = append(checks, checkHistoryDirectory(HistoryDirForConfigPath(path)))

	return DoctorReport{Checks: checks}
}

func checkLMStudio(cfg Config, client *http.Client, timeout time.Duration) Check {
	providers, ok := cfg.Providers["lmstudio"]
	if !ok || len(providers) == 0 {
		return fail("LM Studio endpoint reachable", "no providers.lmstudio entries found", nil)
	}

	var failures []string
	var successes []string
	for name, provider := range providers {
		healthURL, err := lmStudioModelsURL(provider.BaseURL)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			cancel()
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if provider.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+provider.APIKey)
		}

		resp, err := client.Do(req)
		cancel()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			failures = append(failures, fmt.Sprintf("%s: %s returned %s", name, healthURL, resp.Status))
			continue
		}
		successes = append(successes, fmt.Sprintf("%s: %s returned %s", name, healthURL, resp.Status))
	}

	if len(failures) > 0 {
		return fail("LM Studio endpoint reachable", strings.Join(failures, "; "), nil)
	}

	return pass("LM Studio endpoint reachable", strings.Join(successes, "; "))
}

func checkCodex(lookPath func(string) (string, error)) Check {
	path, err := lookPath("codex")
	if err != nil {
		return fail("Codex executable found", "codex", err)
	}

	return pass("Codex executable found", path)
}

func checkWritePermissions(dir string) Check {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fail("write permissions available", dir, err)
	}

	f, err := os.CreateTemp(dir, ".ai-harness-write-check-*")
	if err != nil {
		return fail("write permissions available", dir, err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return fail("write permissions available", name, err)
	}
	if err := os.Remove(name); err != nil {
		return fail("write permissions available", name, err)
	}

	return pass("write permissions available", dir)
}

func checkHistoryDirectory(dir string) Check {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fail("history directory available", dir, err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fail("history directory available", dir, err)
	}
	if !info.IsDir() {
		return fail("history directory available", dir, errors.New("path exists but is not a directory"))
	}

	f, err := os.CreateTemp(dir, ".ai-harness-history-check-*")
	if err != nil {
		return fail("history directory available", dir, err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return fail("history directory available", name, err)
	}
	if err := os.Remove(name); err != nil {
		return fail("history directory available", name, err)
	}

	return pass("history directory available", dir)
}

func lmStudioModelsURL(baseURL string) (string, error) {
	if baseURL == "" {
		return "", errors.New("base_url is empty")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("base_url must include scheme and host: %s", baseURL)
	}

	path := strings.TrimRight(u.Path, "/")
	if path == "" {
		path = "/v1"
	}
	if !strings.HasSuffix(path, "/models") {
		path += "/models"
	}
	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""

	return u.String(), nil
}

func lookPathPortable(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("%s not found on PATH", name)
	}

	exts := []string{"", ".exe", ".cmd", ".bat", ".ps1"}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		for _, ext := range exts {
			candidate := filepath.Join(dir, name+ext)
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("%s not found on PATH", name)
}

func pass(name, message string) Check {
	return Check{Name: name, Status: CheckPass, Message: message}
}

func fail(name, message string, err error) Check {
	return Check{Name: name, Status: CheckFail, Message: message, Err: err}
}
