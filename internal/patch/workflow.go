package patch

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

	repoctx "ai-harness/internal/context"
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args []string, stdin string) (stdout []byte, stderr []byte, err error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, name string, args []string, stdin string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
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

type TestResult struct {
	Command string
	Output  string
	Passed  bool
	Skipped bool
}

func BuildPrompt(task string, snapshot repoctx.Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are generating a safe code patch for this repository.\n\n")
	fmt.Fprintf(&b, "Task:\n%s\n\n", strings.TrimSpace(task))
	fmt.Fprintf(&b, "Return only a unified diff that can be applied with git apply. Do not include markdown fences, prose, or shell commands.\n")
	fmt.Fprintf(&b, "Do not modify secrets, credentials, generated binaries, vendor directories, or unrelated files.\n")
	fmt.Fprintf(&b, "Keep the patch minimal and directly related to the task.\n\n")
	fmt.Fprintf(&b, "Repository context:\n")
	fmt.Fprintf(&b, "Root: %s\n", snapshot.Root)
	if len(snapshot.Languages) > 0 {
		fmt.Fprintf(&b, "Languages: ")
		for i, language := range snapshot.Languages {
			if i > 0 {
				fmt.Fprintf(&b, ", ")
			}
			fmt.Fprintf(&b, "%s", language.Name)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(snapshot.Frameworks) > 0 {
		fmt.Fprintf(&b, "Frameworks: %s\n", strings.Join(snapshot.Frameworks, ", "))
	}
	if snapshot.Git.Status != "" {
		fmt.Fprintf(&b, "Git status:\n%s\n", snapshot.Git.Status)
	}
	if snapshot.Git.Diff != "" {
		fmt.Fprintf(&b, "Git diff:\n%s\n", snapshot.Git.Diff)
		if snapshot.Git.DiffTruncated {
			fmt.Fprintf(&b, "[git diff truncated]\n")
		}
	}
	if len(snapshot.DirectoryStructure) > 0 {
		fmt.Fprintf(&b, "\nDirectory structure:\n")
		for _, entry := range snapshot.DirectoryStructure {
			suffix := ""
			if entry.Type == "dir" {
				suffix = "/"
			}
			fmt.Fprintf(&b, "- %s%s\n", entry.Path, suffix)
		}
	}
	for _, file := range snapshot.KeyFiles {
		fmt.Fprintf(&b, "\nFile: %s", file.Path)
		if file.Truncated {
			fmt.Fprintf(&b, " [truncated]")
		}
		fmt.Fprintf(&b, "\n%s\n", file.Content)
	}
	return b.String()
}

func BuildEscalationPrompt(task string, snapshot repoctx.Snapshot, localFailure string) string {
	prompt := BuildPrompt(task, snapshot)
	localFailure = strings.TrimSpace(localFailure)
	if localFailure == "" {
		return prompt
	}

	var b strings.Builder
	b.WriteString(prompt)
	fmt.Fprintf(&b, "\nEscalation context:\n")
	fmt.Fprintf(&b, "A local LM Studio attempt failed validation or tests.\n")
	fmt.Fprintf(&b, "Failure:\n%s\n\n", localFailure)
	fmt.Fprintf(&b, "If the local patch is already present in the git diff, generate a follow-up unified diff that fixes it.\n")
	fmt.Fprintf(&b, "If no local patch was applied, generate the complete unified diff needed for the task.\n")
	return b.String()
}

func ExtractUnifiedDiff(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("patch response was empty")
	}

	if fenced := extractFencedDiff(raw); fenced != "" {
		raw = fenced
	}

	lines := strings.Split(raw, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "diff --git ") ||
			strings.HasPrefix(trimmed, "--- ") ||
			strings.HasPrefix(trimmed, "Index: ") {
			start = i
			break
		}
	}
	if start == -1 {
		return "", errors.New("response did not contain a unified diff")
	}

	diff := strings.TrimSpace(strings.Join(lines[start:], "\n"))
	if !looksLikeUnifiedDiff(diff) {
		return "", errors.New("response did not contain a valid-looking unified diff")
	}
	return diff + "\n", nil
}

func TouchedFiles(diff string) []string {
	seen := map[string]bool{}
	var files []string
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "--- "):
			files = appendDiffFile(files, seen, strings.TrimSpace(strings.TrimPrefix(line, "--- ")))
		case strings.HasPrefix(line, "+++ "):
			files = appendDiffFile(files, seen, strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
		case strings.HasPrefix(line, "diff --git "):
			for _, field := range strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "diff --git "))) {
				files = appendDiffFile(files, seen, field)
			}
		}
	}
	return files
}

func Validate(root string, diff string, runner Runner) error {
	if runner == nil {
		runner = ExecRunner{}
	}
	_, stderr, err := runner.Run(context.Background(), root, "git", []string{"apply", "--check", "-"}, diff)
	if err != nil {
		return commandError("git apply --check", stderr, err)
	}
	return nil
}

func Apply(ctx context.Context, root string, diff string, runner Runner) error {
	if runner == nil {
		runner = ExecRunner{}
	}
	if _, stderr, err := runner.Run(ctx, root, "git", []string{"apply", "--check", "-"}, diff); err != nil {
		return commandError("git apply --check", stderr, err)
	}
	if _, stderr, err := runner.Run(ctx, root, "git", []string{"apply", "-"}, diff); err != nil {
		return commandError("git apply", stderr, err)
	}
	return nil
}

func RunTests(ctx context.Context, root string, command string, runner Runner) (TestResult, error) {
	if runner == nil {
		runner = ExecRunner{}
	}
	command = strings.TrimSpace(command)
	if command == "" || strings.EqualFold(command, "auto") {
		command = DetectTestCommand(root)
	}
	if command == "" {
		return TestResult{Skipped: true}, nil
	}

	name, args := shellCommand(command)
	stdout, stderr, err := runner.Run(ctx, root, name, args, "")
	output := strings.TrimSpace(strings.TrimSpace(string(stdout)) + "\n" + strings.TrimSpace(string(stderr)))
	result := TestResult{
		Command: command,
		Output:  output,
		Passed:  err == nil,
	}
	if err != nil {
		return result, commandError(command, append(stderr, stdout...), err)
	}
	return result, nil
}

func DetectTestCommand(root string) string {
	if fileExists(filepath.Join(root, "go.mod")) {
		return "go test ./..."
	}
	if fileExists(filepath.Join(root, "package.json")) {
		return "npm test"
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) {
		return "pytest"
	}
	return ""
}

func extractFencedDiff(raw string) string {
	lines := strings.Split(raw, "\n")
	inFence := false
	var captured []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				lower := strings.ToLower(trimmed)
				if lower == "```" || strings.HasPrefix(lower, "```diff") || strings.HasPrefix(lower, "```patch") {
					inFence = true
					captured = nil
				}
				continue
			}
			if len(captured) > 0 {
				return strings.TrimSpace(strings.Join(captured, "\n"))
			}
			inFence = false
			continue
		}
		if inFence {
			captured = append(captured, line)
		}
	}
	return ""
}

func looksLikeUnifiedDiff(diff string) bool {
	lines := strings.Split(diff, "\n")
	hasHeader := false
	hasHunk := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			hasHeader = true
		}
		if strings.HasPrefix(line, "@@") {
			hasHunk = true
		}
	}
	return hasHeader && hasHunk
}

func appendDiffFile(files []string, seen map[string]bool, raw string) []string {
	file := cleanDiffFile(raw)
	if file == "" || seen[file] {
		return files
	}
	seen[file] = true
	return append(files, file)
}

func cleanDiffFile(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "/dev/null" {
		return ""
	}
	if i := strings.IndexByte(raw, '\t'); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.Trim(raw, `"`)
	raw = strings.TrimPrefix(raw, "a/")
	raw = strings.TrimPrefix(raw, "b/")
	if raw == "" || raw == "/dev/null" {
		return ""
	}
	return filepath.ToSlash(raw)
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", command}
	}
	return "sh", []string{"-c", command}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func commandError(command string, output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("%s failed: %s", command, message)
}
