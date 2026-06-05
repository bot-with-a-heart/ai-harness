package repoctx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectBuildsRepositorySnapshot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "# Test Repo\n\nHello.")
	writeTestFile(t, root, "go.mod", "module example.com/test\n\nrequire github.com/spf13/cobra v1.0.0\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n")
	writeTestFile(t, root, "package.json", `{"dependencies":{"react":"latest"},"scripts":{"test":"vitest"}}`)
	writeTestFile(t, root, ".env", "SECRET=do-not-read")
	writeTestFile(t, root, "node_modules/pkg/index.js", "ignored")

	runner := fakeRunner{responses: map[string]fakeCommandResponse{
		"git status --short --branch": {stdout: "## main\n M README.md\n"},
		"git diff --":                 {stdout: "diff --git a/README.md b/README.md\n"},
	}}

	snapshot, err := Collect(context.Background(), Options{
		Root:         root,
		MaxDepth:     4,
		MaxFiles:     50,
		MaxFileBytes: 1024,
		MaxDiffBytes: 1024,
		IncludeDiff:  true,
		Runner:       runner,
		Now:          func() time.Time { return time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if snapshot.Root != root {
		t.Fatalf("root = %q", snapshot.Root)
	}
	if len(snapshot.KeyFiles) != 3 {
		t.Fatalf("key files = %+v", snapshot.KeyFiles)
	}
	if !snapshot.Git.Available || !strings.Contains(snapshot.Git.Status, "README.md") || !strings.Contains(snapshot.Git.Diff, "diff --git") {
		t.Fatalf("git info = %+v", snapshot.Git)
	}
	if !hasLanguage(snapshot, "Go") || !hasLanguage(snapshot, "Markdown") {
		t.Fatalf("languages = %+v", snapshot.Languages)
	}
	for _, framework := range []string{"React", "cobra", "go module", "npm scripts"} {
		if !hasFramework(snapshot, framework) {
			t.Fatalf("framework %q missing from %+v", framework, snapshot.Frameworks)
		}
	}
	for _, entry := range snapshot.DirectoryStructure {
		if strings.Contains(entry.Path, ".env") || strings.Contains(entry.Path, "node_modules") {
			t.Fatalf("sensitive/excluded path leaked into tree: %+v", entry)
		}
	}
}

func TestCollectTruncatesLargeFilesAndDiff(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", strings.Repeat("a", 128))

	runner := fakeRunner{responses: map[string]fakeCommandResponse{
		"git status --short --branch": {stdout: "## main\n"},
		"git diff --":                 {stdout: strings.Repeat("d", 128)},
	}}

	snapshot, err := Collect(context.Background(), Options{
		Root:         root,
		MaxFileBytes: 16,
		MaxDiffBytes: 16,
		IncludeDiff:  true,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(snapshot.KeyFiles) != 1 || !snapshot.KeyFiles[0].Truncated || len(snapshot.KeyFiles[0].Content) != 16 {
		t.Fatalf("key file truncation failed: %+v", snapshot.KeyFiles)
	}
	if !snapshot.Git.DiffTruncated || len(snapshot.Git.Diff) != 16 {
		t.Fatalf("diff truncation failed: %+v", snapshot.Git)
	}
}

func TestCollectRecordsGitWarningWhenUnavailable(t *testing.T) {
	root := t.TempDir()
	runner := fakeRunner{responses: map[string]fakeCommandResponse{
		"git status --short --branch": {stderr: "not a git repository", err: errors.New("exit 128")},
	}}

	snapshot, err := Collect(context.Background(), Options{
		Root:   root,
		Runner: runner,
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if snapshot.Git.Available {
		t.Fatalf("git should be unavailable: %+v", snapshot.Git)
	}
	if len(snapshot.Warnings) == 0 || !strings.Contains(snapshot.Warnings[0], "git status unavailable") {
		t.Fatalf("warnings = %+v", snapshot.Warnings)
	}
}

func writeTestFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func hasLanguage(snapshot Snapshot, name string) bool {
	for _, language := range snapshot.Languages {
		if language.Name == name {
			return true
		}
	}
	return false
}

func hasFramework(snapshot Snapshot, name string) bool {
	for _, framework := range snapshot.Frameworks {
		if framework == name {
			return true
		}
	}
	return false
}

type fakeRunner struct {
	responses map[string]fakeCommandResponse
}

type fakeCommandResponse struct {
	stdout string
	stderr string
	err    error
}

func (r fakeRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, []byte, error) {
	key := name + " " + strings.Join(args, " ")
	response, ok := r.responses[key]
	if !ok {
		return nil, []byte("missing fake response for " + key), errors.New("missing fake response")
	}
	return []byte(response.stdout), []byte(response.stderr), response.err
}
