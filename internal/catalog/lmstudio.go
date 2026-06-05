package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	return stdout, stderr.Bytes(), err
}

type UpdateOptions struct {
	LMSPath          string
	IncludeEstimates bool
	Timeout          time.Duration
	Runner           Runner
	Now              func() time.Time
}

func UpdateFromLMStudio(ctx context.Context, opts UpdateOptions) (Catalog, error) {
	lmsPath := strings.TrimSpace(opts.LMSPath)
	if lmsPath == "" {
		lmsPath = "lms"
		if runtime.GOOS == "windows" {
			lmsPath = "lms.exe"
		}
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	runner := opts.Runner
	if runner == nil {
		runner = ExecRunner{}
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout, stderr, err := runner.Run(ctx, lmsPath, "ls", "--json")
	if err != nil {
		return Catalog{}, commandError("lms ls --json", stderr, err)
	}

	var downloaded []lmStudioModel
	if err := json.Unmarshal(stdout, &downloaded); err != nil {
		return Catalog{}, fmt.Errorf("decode lms ls --json: %w", err)
	}

	loadedByKey := map[string]lmStudioLoaded{}
	stdout, stderr, err = runner.Run(ctx, lmsPath, "ps", "--json")
	warnings := []string{}
	if err != nil {
		warnings = append(warnings, commandError("lms ps --json", stderr, err).Error())
	} else {
		var loaded []lmStudioLoaded
		if err := json.Unmarshal(stdout, &loaded); err != nil {
			warnings = append(warnings, fmt.Sprintf("decode lms ps --json: %v", err))
		} else {
			for _, model := range loaded {
				loadedByKey[model.ModelKey] = model
				if model.Identifier != "" {
					loadedByKey[model.Identifier] = model
				}
			}
		}
	}

	models := make([]Model, 0, len(downloaded))
	for _, item := range downloaded {
		model := item.toCatalogModel()
		if loaded, ok := loadedByKey[item.ModelKey]; ok {
			model.Loaded = true
			if loaded.ContextLength > model.MaxContextLength && model.MaxContextLength == 0 {
				model.MaxContextLength = loaded.ContextLength
			}
		}

		if opts.IncludeEstimates && model.Type != "embedding" && model.ModelKey != "" {
			estimate, err := estimateModel(ctx, runner, lmsPath, model.ModelKey)
			if err != nil {
				warnings = append(warnings, err.Error())
			} else {
				model.EstimatedGPUMemoryGB = estimate.GPUGB
				model.EstimatedTotalMemoryGB = estimate.TotalGB
				model.HardwareFit = estimate.HardwareFit
			}
		}

		models = append(models, scoreModel(model))
	}

	return Catalog{
		UpdatedAt: now().UTC(),
		Source:    "lmstudio",
		Models:    models,
		Warnings:  warnings,
	}, nil
}

func estimateModel(ctx context.Context, runner Runner, lmsPath, modelKey string) (resourceEstimate, error) {
	stdout, stderr, err := runner.Run(ctx, lmsPath, "load", "--estimate-only", modelKey)
	output := strings.TrimSpace(string(stdout))
	if output == "" {
		output = strings.TrimSpace(string(stderr))
	}
	if err != nil && output == "" {
		return resourceEstimate{}, commandError("lms load --estimate-only "+modelKey, stderr, err)
	}

	estimate := parseEstimate(output)
	if estimate.HardwareFit == "unknown" && err != nil {
		estimate.HardwareFit = "too_large"
	}
	if err != nil {
		return estimate, fmt.Errorf("estimate %s: %w", modelKey, commandError("lms load --estimate-only "+modelKey, stderr, err))
	}

	return estimate, nil
}

type resourceEstimate struct {
	GPUGB       float64
	TotalGB     float64
	HardwareFit string
}

var (
	gpuEstimatePattern   = regexp.MustCompile(`(?i)Estimated GPU Memory:\s*([0-9.]+)\s*GiB`)
	totalEstimatePattern = regexp.MustCompile(`(?i)Estimated Total Memory:\s*([0-9.]+)\s*GiB`)
)

func parseEstimate(output string) resourceEstimate {
	estimate := resourceEstimate{
		HardwareFit: "unknown",
	}
	estimate.GPUGB = parseEstimateValue(output, gpuEstimatePattern)
	estimate.TotalGB = parseEstimateValue(output, totalEstimatePattern)

	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "may be loaded") || strings.Contains(lower, "can be loaded"):
		estimate.HardwareFit = "fits"
	case strings.Contains(lower, "may not be loaded") ||
		strings.Contains(lower, "cannot be loaded") ||
		strings.Contains(lower, "not enough") ||
		strings.Contains(lower, "exceed"):
		estimate.HardwareFit = "too_large"
	case strings.Contains(lower, "close") || strings.Contains(lower, "guardrail"):
		estimate.HardwareFit = "borderline"
	}

	return estimate
}

func parseEstimateValue(output string, pattern *regexp.Regexp) float64 {
	matches := pattern.FindStringSubmatch(output)
	if len(matches) != 2 {
		return 0
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	return value
}

func commandError(command string, stderr []byte, err error) error {
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("%s failed: %s", command, message)
}

type lmStudioModel struct {
	Type                   string       `json:"type"`
	ModelKey               string       `json:"modelKey"`
	DisplayName            string       `json:"displayName"`
	Publisher              string       `json:"publisher"`
	Path                   string       `json:"path"`
	SizeBytes              int64        `json:"sizeBytes"`
	IndexedModelIdentifier string       `json:"indexedModelIdentifier"`
	ParamsString           string       `json:"paramsString"`
	Architecture           string       `json:"architecture"`
	Quantization           quantization `json:"quantization"`
	Vision                 bool         `json:"vision"`
	TrainedForToolUse      bool         `json:"trainedForToolUse"`
	MaxContextLength       int          `json:"maxContextLength"`
}

type lmStudioLoaded struct {
	ModelKey      string `json:"modelKey"`
	Identifier    string `json:"identifier"`
	ContextLength int    `json:"contextLength"`
	Status        string `json:"status"`
}

type quantization struct {
	Name string `json:"name"`
	Bits int    `json:"bits"`
}

func (m lmStudioModel) toCatalogModel() Model {
	id := firstNonEmpty(m.ModelKey, m.IndexedModelIdentifier, m.Path, m.DisplayName)
	hardwareFit := "unknown"
	if m.Type == "embedding" {
		hardwareFit = "fits"
	}

	return Model{
		ID:                id,
		ModelKey:          firstNonEmpty(m.ModelKey, id),
		DisplayName:       firstNonEmpty(m.DisplayName, id),
		Type:              firstNonEmpty(m.Type, "unknown"),
		Downloaded:        true,
		Architecture:      m.Architecture,
		Params:            m.ParamsString,
		SizeBytes:         m.SizeBytes,
		MaxContextLength:  m.MaxContextLength,
		TrainedForToolUse: m.TrainedForToolUse,
		Vision:            m.Vision,
		HardwareFit:       hardwareFit,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
