package repoctx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Snapshot struct {
	Root               string         `json:"root"`
	GeneratedAt        time.Time      `json:"generatedAt"`
	KeyFiles           []KeyFile      `json:"keyFiles"`
	Git                GitInfo        `json:"git"`
	DirectoryStructure []TreeEntry    `json:"directoryStructure"`
	Languages          []LanguageInfo `json:"languages"`
	Frameworks         []string       `json:"frameworks"`
	ExcludedPatterns   []string       `json:"excludedPatterns"`
	Warnings           []string       `json:"warnings,omitempty"`
}

type KeyFile struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

type GitInfo struct {
	Available     bool   `json:"available"`
	Status        string `json:"status,omitempty"`
	Diff          string `json:"diff,omitempty"`
	DiffTruncated bool   `json:"diffTruncated,omitempty"`
}

type TreeEntry struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Depth int    `json:"depth"`
}

type LanguageInfo struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

type Options struct {
	Root         string
	MaxDepth     int
	MaxFiles     int
	MaxFileBytes int
	MaxDiffBytes int
	IncludeDiff  bool
	Runner       Runner
	Now          func() time.Time
}

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	return stdout, stderr.Bytes(), err
}

var keyFileNames = []string{
	"README.md",
	"readme.md",
	"README",
	"package.json",
	"go.mod",
	"pyproject.toml",
}

var excludedNames = map[string]bool{
	".git":          true,
	".hg":           true,
	".svn":          true,
	".ai-harness":   true,
	".codex":        true,
	".gocache":      true,
	".gotmp":        true,
	"node_modules":  true,
	"vendor":        true,
	"dist":          true,
	"build":         true,
	"target":        true,
	"coverage":      true,
	".next":         true,
	".nuxt":         true,
	".venv":         true,
	"venv":          true,
	"__pycache__":   true,
	".pytest_cache": true,
	"bin":           true,
}

var excludedFragments = []string{
	".env",
	"secret",
	"secrets",
	"credential",
	"credentials",
	"private_key",
	"id_rsa",
	"id_ed25519",
	".pem",
	".p12",
	".pfx",
	".key",
	".exe",
}

var extensionLanguages = map[string]string{
	".go":    "Go",
	".js":    "JavaScript",
	".jsx":   "JavaScript",
	".ts":    "TypeScript",
	".tsx":   "TypeScript",
	".py":    "Python",
	".java":  "Java",
	".kt":    "Kotlin",
	".rs":    "Rust",
	".cs":    "C#",
	".cpp":   "C++",
	".cxx":   "C++",
	".cc":    "C++",
	".c":     "C",
	".h":     "C/C++ Header",
	".hpp":   "C/C++ Header",
	".rb":    "Ruby",
	".php":   "PHP",
	".swift": "Swift",
	".html":  "HTML",
	".css":   "CSS",
	".scss":  "CSS",
	".json":  "JSON",
	".toml":  "TOML",
	".yaml":  "YAML",
	".yml":   "YAML",
	".md":    "Markdown",
}

func Collect(ctx context.Context, opts Options) (Snapshot, error) {
	root, err := resolveRoot(opts.Root)
	if err != nil {
		return Snapshot{}, err
	}
	opts = withDefaults(opts)

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	snapshot := Snapshot{
		Root:             root,
		GeneratedAt:      now().UTC(),
		ExcludedPatterns: excludedPatternDescriptions(),
	}

	keyFiles, warnings := collectKeyFiles(root, opts.MaxFileBytes)
	snapshot.KeyFiles = keyFiles
	snapshot.Warnings = append(snapshot.Warnings, warnings...)

	tree, languages, treeWarnings := collectTreeAndLanguages(root, opts.MaxDepth, opts.MaxFiles)
	snapshot.DirectoryStructure = tree
	snapshot.Languages = languages
	snapshot.Warnings = append(snapshot.Warnings, treeWarnings...)

	git, gitWarnings := collectGit(ctx, root, opts)
	snapshot.Git = git
	snapshot.Warnings = append(snapshot.Warnings, gitWarnings...)

	snapshot.Frameworks = detectFrameworks(snapshot.KeyFiles)

	return snapshot, nil
}

func withDefaults(opts Options) Options {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 3
	}
	if opts.MaxFiles <= 0 {
		opts.MaxFiles = 300
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = 16 * 1024
	}
	if opts.MaxDiffBytes <= 0 {
		opts.MaxDiffBytes = 64 * 1024
	}
	if opts.Runner == nil {
		opts.Runner = ExecRunner{}
	}
	return opts
}

func resolveRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve repository path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect repository path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository path is not a directory: %s", abs)
	}
	return abs, nil
}

func collectKeyFiles(root string, maxBytes int) ([]KeyFile, []string) {
	var files []KeyFile
	var warnings []string
	seen := map[string]bool{}

	for _, name := range keyFileNames {
		if seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || shouldExclude(name) {
			continue
		}
		content, truncated, err := readLimited(path, maxBytes)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read %s: %v", name, err))
			continue
		}
		files = append(files, KeyFile{
			Path:      filepath.ToSlash(name),
			Content:   content,
			Truncated: truncated,
		})
	}

	return files, warnings
}

func collectTreeAndLanguages(root string, maxDepth int, maxFiles int) ([]TreeEntry, []LanguageInfo, []string) {
	var entries []TreeEntry
	var warnings []string
	languages := map[string]*LanguageInfo{}
	visited := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("walk %s: %v", path, err))
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("rel %s: %v", path, err))
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if shouldExclude(relSlash) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		depth := pathDepth(relSlash)
		if d.IsDir() && depth > maxDepth {
			return filepath.SkipDir
		}
		if !d.IsDir() && depth > maxDepth {
			return nil
		}
		if visited >= maxFiles {
			return filepath.SkipDir
		}
		visited++

		entryType := "file"
		if d.IsDir() {
			entryType = "dir"
		}
		entries = append(entries, TreeEntry{
			Path:  relSlash,
			Type:  entryType,
			Depth: depth,
		})

		if !d.IsDir() {
			info, statErr := d.Info()
			if statErr == nil {
				addLanguage(languages, relSlash, info.Size())
			}
		}
		return nil
	})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("walk repository: %v", err))
	}
	if visited >= maxFiles {
		warnings = append(warnings, fmt.Sprintf("directory structure truncated at %d entries", maxFiles))
	}

	return entries, sortedLanguages(languages), warnings
}

func collectGit(ctx context.Context, root string, opts Options) (GitInfo, []string) {
	var warnings []string
	status, stderr, err := opts.Runner.Run(ctx, root, "git", "status", "--short", "--branch")
	if err != nil {
		msg := strings.TrimSpace(string(stderr))
		if msg == "" {
			msg = err.Error()
		}
		warnings = append(warnings, "git status unavailable: "+msg)
		return GitInfo{Available: false}, warnings
	}

	git := GitInfo{
		Available: true,
		Status:    strings.TrimSpace(string(status)),
	}

	if opts.IncludeDiff {
		diff, stderr, err := opts.Runner.Run(ctx, root, "git", "diff", "--")
		if err != nil {
			msg := strings.TrimSpace(string(stderr))
			if msg == "" {
				msg = err.Error()
			}
			warnings = append(warnings, "git diff unavailable: "+msg)
		} else {
			git.Diff, git.DiffTruncated = truncateString(string(diff), opts.MaxDiffBytes)
		}
	}

	return git, warnings
}

func readLimited(path string, maxBytes int) (string, bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	content, truncated := truncateString(string(contents), maxBytes)
	return content, truncated, nil
}

func truncateString(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	return value[:maxBytes], true
}

func shouldExclude(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	parts := strings.Split(relPath, "/")
	for _, part := range parts {
		lower := strings.ToLower(part)
		if excludedNames[lower] {
			return true
		}
		for _, fragment := range excludedFragments {
			if strings.Contains(lower, fragment) {
				return true
			}
		}
	}
	return false
}

func pathDepth(relPath string) int {
	if relPath == "" {
		return 0
	}
	return strings.Count(filepath.ToSlash(relPath), "/") + 1
}

func addLanguage(languages map[string]*LanguageInfo, relPath string, size int64) {
	ext := strings.ToLower(filepath.Ext(relPath))
	name, ok := extensionLanguages[ext]
	if !ok {
		return
	}
	info, ok := languages[name]
	if !ok {
		info = &LanguageInfo{Name: name}
		languages[name] = info
	}
	info.Files++
	info.Bytes += size
}

func sortedLanguages(languages map[string]*LanguageInfo) []LanguageInfo {
	out := make([]LanguageInfo, 0, len(languages))
	for _, info := range languages {
		out = append(out, *info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Files == out[j].Files {
			return out[i].Name < out[j].Name
		}
		return out[i].Files > out[j].Files
	})
	return out
}

func detectFrameworks(files []KeyFile) []string {
	seen := map[string]bool{}
	for _, file := range files {
		switch strings.ToLower(file.Path) {
		case "go.mod":
			detectGoFrameworks(file.Content, seen)
		case "package.json":
			detectPackageJSONFrameworks(file.Content, seen)
		case "pyproject.toml":
			detectPyProjectFrameworks(file.Content, seen)
		}
	}

	out := make([]string, 0, len(seen))
	for framework := range seen {
		out = append(out, framework)
	}
	sort.Strings(out)
	return out
}

func detectGoFrameworks(content string, seen map[string]bool) {
	checks := map[string]string{
		"github.com/spf13/cobra":       "cobra",
		"github.com/knadh/koanf":       "koanf",
		"github.com/rs/zerolog":        "zerolog",
		"github.com/stretchr/testify":  "testify",
		"github.com/gin-gonic/gin":     "gin",
		"github.com/labstack/echo":     "echo",
		"github.com/gofiber/fiber":     "fiber",
		"go-playground/validator":      "go-playground/validator",
		"github.com/pelletier/go-toml": "go-toml",
	}
	for needle, name := range checks {
		if strings.Contains(content, needle) {
			seen[name] = true
		}
	}
	if strings.Contains(content, "\nmodule ") || strings.HasPrefix(content, "module ") {
		seen["go module"] = true
	}
}

func detectPackageJSONFrameworks(content string, seen map[string]bool) {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return
	}
	deps := map[string]bool{}
	for name := range pkg.Dependencies {
		deps[name] = true
	}
	for name := range pkg.DevDependencies {
		deps[name] = true
	}
	checks := map[string]string{
		"react":         "React",
		"next":          "Next.js",
		"vue":           "Vue",
		"nuxt":          "Nuxt",
		"@angular/core": "Angular",
		"svelte":        "Svelte",
		"vite":          "Vite",
		"express":       "Express",
		"jest":          "Jest",
		"vitest":        "Vitest",
		"typescript":    "TypeScript",
	}
	for dep, name := range checks {
		if deps[dep] {
			seen[name] = true
		}
	}
	for script := range pkg.Scripts {
		if strings.Contains(strings.ToLower(script), "test") {
			seen["npm scripts"] = true
		}
	}
}

func detectPyProjectFrameworks(content string, seen map[string]bool) {
	var pyproject map[string]any
	if err := toml.Unmarshal([]byte(content), &pyproject); err != nil {
		return
	}
	text := strings.ToLower(content)
	checks := map[string]string{
		"django":  "Django",
		"fastapi": "FastAPI",
		"flask":   "Flask",
		"pytest":  "pytest",
		"poetry":  "Poetry",
		"ruff":    "Ruff",
	}
	for needle, name := range checks {
		if strings.Contains(text, needle) {
			seen[name] = true
		}
	}
}

func excludedPatternDescriptions() []string {
	out := make([]string, 0, len(excludedNames)+len(excludedFragments))
	for name := range excludedNames {
		out = append(out, name)
	}
	out = append(out, excludedFragments...)
	sort.Strings(out)
	return out
}

func ValidateSnapshot(snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.Root) == "" {
		return errors.New("snapshot root is required")
	}
	return nil
}
