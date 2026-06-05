package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/providers"
)

const defaultTimeout = 10 * time.Minute

type Client struct {
	name       string
	executable string
	profile    string
	workingDir string
	sandbox    string
	runner     Runner
}

type Runner interface {
	Run(ctx context.Context, name string, args []string, stdin string) (stdout []byte, stderr []byte, err error)
	LookPath(name string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (ExecRunner) LookPath(name string) (string, error) {
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

type Option func(*Client)

func WithRunner(runner Runner) Option {
	return func(c *Client) {
		c.runner = runner
	}
}

func WithWorkingDir(dir string) Option {
	return func(c *Client) {
		c.workingDir = dir
	}
}

func WithSandbox(sandbox string) Option {
	return func(c *Client) {
		c.sandbox = sandbox
	}
}

func New(name string, cfg appconfig.Provider, opts ...Option) (*Client, error) {
	if name == "" {
		name = "default"
	}
	profile := strings.TrimSpace(cfg.Profile)

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}

	client := &Client{
		name:       name,
		executable: "codex",
		profile:    profile,
		workingDir: workingDir,
		sandbox:    "read-only",
		runner:     ExecRunner{},
	}
	for _, opt := range opts {
		opt(client)
	}
	if client.runner == nil {
		client.runner = ExecRunner{}
	}

	return client, nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Health(ctx context.Context) error {
	_, err := c.runner.LookPath(c.executable)
	if err != nil {
		return err
	}
	_, stderr, err := c.runner.Run(ctx, c.executable, []string{"--version"}, "")
	if err != nil {
		return commandError("codex --version", stderr, err)
	}
	return nil
}

func (c *Client) ListModels(context.Context) ([]providers.Model, error) {
	return nil, errors.New("codex provider does not support model discovery")
}

func (c *Client) Ask(ctx context.Context, req providers.AskRequest) (providers.AskResponse, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return providers.AskResponse{}, errors.New("prompt is required")
	}
	if _, err := c.runner.LookPath(c.executable); err != nil {
		return providers.AskResponse{}, err
	}

	outputFile, cleanup, err := tempOutputFile()
	if err != nil {
		return providers.AskResponse{}, err
	}
	defer cleanup()

	args := []string{
		"exec",
		"--cd", c.workingDir,
		"--sandbox", c.sandbox,
		"--ephemeral",
		"--color", "never",
		"--output-last-message", outputFile,
	}
	if c.profile != "" && c.profile != "default" {
		args = append([]string{"exec", "--profile", c.profile}, args[1:]...)
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	args = append(args, prompt)

	stdout, stderr, err := c.runner.Run(ctx, c.executable, args, "")

	content := readLastMessage(outputFile)
	if content == "" {
		content = strings.TrimSpace(string(stdout))
	}
	if content == "" {
		content = strings.TrimSpace(string(stderr))
	}
	if err != nil && content == "" {
		return providers.AskResponse{}, commandError("codex exec", append(stderr, stdout...), err)
	}
	if content == "" {
		return providers.AskResponse{}, errors.New("codex exec returned no output")
	}

	model := req.Model
	if model == "" {
		model = "codex-cli"
	}

	return providers.AskResponse{
		Model:   model,
		Content: content,
	}, nil
}

func tempOutputFile() (string, func(), error) {
	f, err := os.CreateTemp("", "ai-harness-codex-last-message-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("create codex output file: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", nil, fmt.Errorf("close codex output file: %w", err)
	}
	return name, func() { _ = os.Remove(name) }, nil
}

func readLastMessage(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(contents))
}

func commandError(command string, output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("%s failed: %s", command, message)
}

func ContextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return context.WithTimeout(ctx, timeout)
}
