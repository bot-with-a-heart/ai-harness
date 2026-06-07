package obsidian

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-harness/internal/catalog"
	appconfig "ai-harness/internal/config"
	"ai-harness/internal/history"
)

const managedMarker = "ai_harness_managed: true"

type ExportOptions struct {
	VaultPath       string
	Folder          string
	Catalog         *catalog.Catalog
	History         []history.Record
	IncludeHistory  bool
	IncludeTaskText bool
	Now             func() time.Time
}

type Plan struct {
	VaultPath string
	Folder    string
	FolderDir string
	Files     []File
	Warnings  []string
}

type File struct {
	RelativePath string
	Path         string
	Contents     string
	Exists       bool
	Conflict     bool
}

func BuildPlan(opts ExportOptions) (Plan, error) {
	vaultPath := strings.TrimSpace(opts.VaultPath)
	if vaultPath == "" {
		return Plan{}, errors.New("Obsidian vault path is required")
	}
	folder := strings.TrimSpace(opts.Folder)
	if folder == "" {
		folder = appconfig.DefaultObsidianFolder
	}

	folderDir, err := safeJoin(vaultPath, folder)
	if err != nil {
		return Plan{}, err
	}

	generatedAt := now(opts.Now)
	plan := Plan{
		VaultPath: vaultPath,
		Folder:    folder,
		FolderDir: folderDir,
	}
	plan.Files = append(plan.Files, buildFile(vaultPath, folder, "Index.md", indexNote(generatedAt, opts.IncludeHistory)))

	if opts.Catalog != nil {
		plan.Files = append(plan.Files, buildFile(vaultPath, folder, "Models/Model Catalog.md", modelCatalogNote(generatedAt, *opts.Catalog)))
	}

	if opts.IncludeHistory {
		plan.Files = append(plan.Files, buildFile(vaultPath, folder, "History/Recent Task Outcomes.md", historyNote(generatedAt, opts.History, opts.IncludeTaskText)))
	}

	for i := range plan.Files {
		exists, conflict, err := inspectExisting(plan.Files[i].Path)
		if err != nil {
			return Plan{}, err
		}
		plan.Files[i].Exists = exists
		plan.Files[i].Conflict = conflict
	}

	return plan, nil
}

func WritePlan(plan Plan, force bool) error {
	for _, file := range plan.Files {
		if file.Conflict && !force {
			return fmt.Errorf("refusing to overwrite user-edited Obsidian note %s; re-run with --force to replace it", file.RelativePath)
		}
	}
	for _, file := range plan.Files {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o700); err != nil {
			return fmt.Errorf("create Obsidian note directory: %w", err)
		}
		if err := os.WriteFile(file.Path, []byte(file.Contents), 0o600); err != nil {
			return fmt.Errorf("write Obsidian note %s: %w", file.RelativePath, err)
		}
	}
	return nil
}

func HasConflicts(plan Plan) bool {
	for _, file := range plan.Files {
		if file.Conflict {
			return true
		}
	}
	return false
}

func buildFile(vaultPath string, folder string, rel string, contents string) File {
	fullRel := filepath.Join(folder, filepath.FromSlash(rel))
	path, _ := safeJoin(vaultPath, fullRel)
	return File{
		RelativePath: filepath.ToSlash(fullRel),
		Path:         path,
		Contents:     contents,
	}
}

func inspectExisting(path string) (exists bool, conflict bool, err error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("inspect existing Obsidian note: %w", err)
	}
	return true, !strings.Contains(string(contents), managedMarker), nil
}

func safeJoin(root string, rel string) (string, error) {
	root = strings.TrimSpace(root)
	rel = strings.TrimSpace(rel)
	if root == "" {
		return "", errors.New("root path is required")
	}
	if rel == "" || rel == "." {
		return filepath.Clean(root), nil
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("Obsidian relative path %q must not be absolute", rel)
	}
	for _, part := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("Obsidian relative path %q must not escape the vault", rel)
		}
	}
	return filepath.Join(root, filepath.Clean(rel)), nil
}

func now(fn func() time.Time) time.Time {
	if fn != nil {
		return fn().UTC()
	}
	return time.Now().UTC()
}

func frontmatter(kind string, generatedAt time.Time) string {
	return fmt.Sprintf("---\n%s\ntype: %s\ngenerated_at: %s\n---\n\n", managedMarker, kind, generatedAt.Format(time.RFC3339))
}

func indexNote(generatedAt time.Time, includeHistory bool) string {
	var b strings.Builder
	b.WriteString(frontmatter("index", generatedAt))
	b.WriteString("# AI Harness Memory\n\n")
	b.WriteString("This folder contains AI Harness exports for human review in Obsidian.\n\n")
	b.WriteString("Canonical records remain in AI Harness storage. These notes are generated copies, not the source of truth.\n\n")
	b.WriteString("## Notes\n\n")
	b.WriteString("- [[Models/Model Catalog|Model Catalog]]\n")
	if includeHistory {
		b.WriteString("- [[History/Recent Task Outcomes|Recent Task Outcomes]]\n")
	}
	return b.String()
}

func modelCatalogNote(generatedAt time.Time, modelCatalog catalog.Catalog) string {
	models := append([]catalog.Model(nil), modelCatalog.Models...)
	sort.SliceStable(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	var b strings.Builder
	b.WriteString(frontmatter("model_catalog", generatedAt))
	b.WriteString("# Model Catalog\n\n")
	if !modelCatalog.UpdatedAt.IsZero() {
		fmt.Fprintf(&b, "Updated: %s\n\n", modelCatalog.UpdatedAt.Format(time.RFC3339))
	}
	if modelCatalog.Source != "" {
		fmt.Fprintf(&b, "Source: %s\n\n", modelCatalog.Source)
	}
	b.WriteString("| Model | Loaded | Fit | Speed | Complexity | Best For |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, model := range models {
		fmt.Fprintf(
			&b,
			"| %s | %t | %s | %s | %s | %s |\n",
			mdCell(model.ID),
			model.Loaded,
			mdCell(model.HardwareFit),
			mdCell(model.SpeedClass),
			mdCell(model.ComplexityClass),
			mdCell(strings.Join(model.BestFor, ", ")),
		)
	}
	if len(modelCatalog.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, warning := range modelCatalog.Warnings {
			fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(warning))
		}
	}
	return b.String()
}

func historyNote(generatedAt time.Time, records []history.Record, includeTaskText bool) string {
	var b strings.Builder
	b.WriteString(frontmatter("task_outcomes", generatedAt))
	b.WriteString("# Recent Task Outcomes\n\n")
	if !includeTaskText {
		b.WriteString("Task text is redacted by default. Re-export with `--include-task-text` to include it.\n\n")
	}
	if len(records) == 0 {
		b.WriteString("No history records were available.\n")
		return b.String()
	}
	for _, record := range records {
		title := record.Command
		if title == "" {
			title = "execution"
		}
		fmt.Fprintf(&b, "## %s - %s\n\n", record.Timestamp.Format(time.RFC3339), title)
		fmt.Fprintf(&b, "- Status: %s\n", empty(record.Status))
		fmt.Fprintf(&b, "- Provider: %s\n", empty(record.Provider))
		fmt.Fprintf(&b, "- Model: %s\n", empty(record.Model))
		fmt.Fprintf(&b, "- Escalated: %t\n", record.Escalated)
		if includeTaskText {
			fmt.Fprintf(&b, "- Task: %s\n", strings.TrimSpace(record.Task))
		} else {
			b.WriteString("- Task: redacted\n")
		}
		if len(record.FilesTouched) > 0 {
			fmt.Fprintf(&b, "- Files touched: %s\n", strings.Join(record.FilesTouched, ", "))
		}
		if len(record.TestsRun) > 0 {
			tests := make([]string, 0, len(record.TestsRun))
			for _, test := range record.TestsRun {
				label := test.Command
				if label == "" && test.Skipped {
					label = "skipped"
				}
				if test.Passed {
					label += " passed"
				} else if test.Skipped {
					label += " skipped"
				} else {
					label += " failed"
				}
				tests = append(tests, strings.TrimSpace(label))
			}
			fmt.Fprintf(&b, "- Tests: %s\n", strings.Join(tests, "; "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func mdCell(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	if value == "" {
		return "-"
	}
	return value
}

func empty(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
