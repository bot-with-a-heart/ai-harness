package catalog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var paramsPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*([bmk])`)

func scoreModel(model Model) Model {
	model.HardwareFit = normalizeDefault(model.HardwareFit, "unknown")
	model.SpeedClass = "unknown"
	model.ComplexityClass = "unknown"
	model.BestFor = nil
	model.AvoidFor = nil

	name := strings.ToLower(strings.Join([]string{
		model.ID,
		model.ModelKey,
		model.DisplayName,
		model.Architecture,
		model.Params,
	}, " "))

	if model.Type == "embedding" || strings.Contains(name, "embedding") || strings.Contains(name, "embed") {
		model.SpeedClass = "fast"
		model.ComplexityClass = "low"
		model.BestFor = []string{"retrieval", "semantic search", "repository context indexing"}
		model.AvoidFor = []string{"chat responses", "code edits", "multi-step reasoning"}
		model.Notes = "Embedding model; use for retrieval and context support rather than coding conversations."
		return model
	}

	paramsB := paramsInBillions(model.Params)
	switch {
	case paramsB >= 30 || model.SizeBytes >= 30_000_000_000:
		model.SpeedClass = "slow"
		model.ComplexityClass = "high"
	case paramsB >= 10 || model.SizeBytes >= 8_000_000_000:
		model.SpeedClass = "balanced"
		model.ComplexityClass = "medium"
	case paramsB > 0 || model.SizeBytes > 0:
		model.SpeedClass = "fast"
		model.ComplexityClass = "low"
	default:
		model.SpeedClass = "unknown"
		model.ComplexityClass = "unknown"
	}

	bestFor := []string{"explanation", "code review"}
	avoidFor := []string{}

	if strings.Contains(name, "coder") || strings.Contains(name, "code") {
		bestFor = append(bestFor, "code generation", "test generation", "small to medium edits")
		if model.ComplexityClass == "high" {
			bestFor = append(bestFor, "large refactors")
		}
	}
	if strings.Contains(name, "reason") || strings.Contains(name, "thinking") {
		model.ComplexityClass = maxComplexity(model.ComplexityClass, "high")
		bestFor = append(bestFor, "debugging", "multi-step reasoning")
	}
	if model.TrainedForToolUse {
		bestFor = append(bestFor, "tool-assisted workflows")
	}
	if model.Vision {
		bestFor = append(bestFor, "image-aware tasks")
	}
	if model.MaxContextLength >= 64_000 {
		bestFor = append(bestFor, "large context analysis")
	}
	if model.HardwareFit == "too_large" {
		avoidFor = append(avoidFor, "local execution without explicit user approval")
	}
	if model.SpeedClass == "slow" {
		avoidFor = append(avoidFor, "quick answers when a smaller model is sufficient")
	}
	if model.ComplexityClass == "low" {
		avoidFor = append(avoidFor, "large refactors", "complex debugging")
	}

	model.BestFor = uniqueStrings(bestFor)
	model.AvoidFor = uniqueStrings(avoidFor)
	model.Notes = summaryFor(model)

	return model
}

func paramsInBillions(params string) float64 {
	matches := paramsPattern.FindStringSubmatch(params)
	if len(matches) != 3 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	switch strings.ToLower(matches[2]) {
	case "b":
		return value
	case "m":
		return value / 1000
	case "k":
		return value / 1_000_000
	default:
		return 0
	}
}

func maxComplexity(current, candidate string) string {
	rank := map[string]int{
		"unknown": 0,
		"low":     1,
		"medium":  2,
		"high":    3,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}

	return out
}

func normalizeDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func summaryFor(model Model) string {
	parts := []string{
		fmt.Sprintf("%s speed", model.SpeedClass),
		fmt.Sprintf("%s complexity", model.ComplexityClass),
		fmt.Sprintf("%s hardware fit", model.HardwareFit),
	}
	if model.Loaded {
		parts = append(parts, "currently loaded")
	}
	if model.TrainedForToolUse {
		parts = append(parts, "tool-use trained")
	}
	if model.Vision {
		parts = append(parts, "vision-capable")
	}

	return strings.Join(parts, "; ") + "."
}
