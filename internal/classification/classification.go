package classification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ai-harness/internal/providers"
)

type Decision struct {
	Complexity          string `json:"complexity"`
	Risk                string `json:"risk"`
	NeedsRepoAccess     bool   `json:"needsRepoAccess"`
	NeedsEdits          bool   `json:"needsEdits"`
	NeedsTests          bool   `json:"needsTests"`
	RecommendedProvider string `json:"recommendedProvider"`
	Reason              string `json:"reason"`
}

type Request struct {
	Task  string
	Model string
}

type Agent struct {
	Provider providers.Provider
}

func (a Agent) Classify(ctx context.Context, req Request) (Decision, error) {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return Decision{}, errors.New("task is required")
	}
	if a.Provider == nil {
		return Decision{}, errors.New("classification provider is required")
	}

	response, err := a.Provider.Ask(ctx, providers.AskRequest{
		Model:  req.Model,
		Prompt: BuildPrompt(task),
	})
	if err != nil {
		return Decision{}, fmt.Errorf("ask classification provider: %w", err)
	}

	decision, err := Parse(response.Content)
	if err != nil {
		return Decision{}, err
	}

	return decision, nil
}

func BuildPrompt(task string) string {
	return fmt.Sprintf(`You are the routing classifier for a local-first AI coding harness.

Classify the user's software development task and choose whether it should run on LM Studio or Codex.

Use LM Studio for:
- explanations
- documentation
- code review
- small functions
- simple test generation

Use Codex for:
- multi-file changes
- architecture work
- security-sensitive changes
- complex debugging
- failed local attempts

Return only valid JSON. Do not use markdown. Do not include extra keys.

Schema:
{
  "complexity": "low|medium|high",
  "risk": "low|medium|high",
  "needsRepoAccess": true,
  "needsEdits": false,
  "needsTests": false,
  "recommendedProvider": "lmstudio|codex",
  "reason": "short reason"
}

Task:
%s`, task)
}

func Parse(raw string) (Decision, error) {
	jsonText, err := extractJSONObject(raw)
	if err != nil {
		return Decision{}, err
	}

	var decision Decision
	if err := json.Unmarshal([]byte(jsonText), &decision); err != nil {
		return Decision{}, fmt.Errorf("parse classification JSON: %w", err)
	}

	if err := normalizeAndValidate(&decision); err != nil {
		return Decision{}, err
	}

	return decision, nil
}

func Heuristic(task string) (Decision, error) {
	task = strings.TrimSpace(task)
	if task == "" {
		return Decision{}, errors.New("task is required")
	}

	lower := strings.ToLower(task)
	decision := Decision{
		Complexity:          "low",
		Risk:                "low",
		NeedsRepoAccess:     needsRepoAccess(lower),
		NeedsEdits:          needsEdits(lower),
		NeedsTests:          needsTests(lower),
		RecommendedProvider: "lmstudio",
		Reason:              "Simple local-first task.",
	}

	if containsAny(lower, "architecture", "refactor", "multi-file", "migrate", "redesign", "aws cdk", "terraform", "deployment") {
		decision.Complexity = "high"
		decision.NeedsRepoAccess = true
		decision.RecommendedProvider = "codex"
		decision.Reason = "Architecture or multi-file work should use Codex."
	}
	if containsAny(lower, "debug", "failing test", "flaky", "race condition", "deadlock", "performance regression") {
		decision.Complexity = maxLevel(decision.Complexity, "high")
		decision.NeedsRepoAccess = true
		decision.NeedsTests = true
		decision.RecommendedProvider = "codex"
		decision.Reason = "Complex debugging usually needs repo access and test execution."
	}
	if containsAny(lower, "security", "auth", "authentication", "authorization", "crypto", "secret", "permission", "vulnerability") {
		decision.Risk = "high"
		decision.Complexity = maxLevel(decision.Complexity, "medium")
		decision.NeedsRepoAccess = true
		decision.RecommendedProvider = "codex"
		decision.Reason = "Security-sensitive work should use Codex."
	}
	if containsAny(lower, "add tests", "unit test", "test generation") && decision.Complexity != "high" && decision.Risk != "high" {
		decision.Complexity = maxLevel(decision.Complexity, "medium")
		decision.NeedsTests = true
		decision.Reason = "Test generation is suitable for the local provider unless it spans many files."
	}
	if containsAny(lower, "explain", "summarize", "document", "readme", "comment", "review") && !decision.NeedsEdits && decision.Risk != "high" {
		decision.RecommendedProvider = "lmstudio"
		decision.Reason = "Explanation, documentation, or review can start locally."
	}

	return decision, nil
}

func extractJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("classification response was empty")
	}

	start := strings.Index(raw, "{")
	if start == -1 {
		return "", fmt.Errorf("classification response did not contain JSON object: %s", raw)
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1], nil
			}
		}
	}

	return "", errors.New("classification response contained incomplete JSON object")
}

func normalizeAndValidate(decision *Decision) error {
	decision.Complexity = strings.ToLower(strings.TrimSpace(decision.Complexity))
	decision.Risk = strings.ToLower(strings.TrimSpace(decision.Risk))
	decision.RecommendedProvider = strings.ToLower(strings.TrimSpace(decision.RecommendedProvider))
	decision.Reason = strings.TrimSpace(decision.Reason)

	if !validLevel(decision.Complexity) {
		return fmt.Errorf("invalid complexity %q", decision.Complexity)
	}
	if !validLevel(decision.Risk) {
		return fmt.Errorf("invalid risk %q", decision.Risk)
	}
	if decision.RecommendedProvider != "lmstudio" && decision.RecommendedProvider != "codex" {
		return fmt.Errorf("invalid recommendedProvider %q", decision.RecommendedProvider)
	}
	if decision.Reason == "" {
		return errors.New("classification reason is required")
	}

	return nil
}

func validLevel(value string) bool {
	return value == "low" || value == "medium" || value == "high"
}

func needsRepoAccess(task string) bool {
	return containsAny(task, "repo", "repository", "codebase", "project", "package", "module", "middleware", "file", "app", "application", "review", "refactor", "debug", "tests")
}

func needsEdits(task string) bool {
	return containsAny(task, "add", "change", "edit", "fix", "implement", "refactor", "write", "update", "modify", "migrate")
}

func needsTests(task string) bool {
	return containsAny(task, "test", "tests", "failing", "bug", "fix", "debug", "regression")
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func maxLevel(a, b string) string {
	rank := map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
