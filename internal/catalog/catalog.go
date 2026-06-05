package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appconfig "ai-harness/internal/config"
)

const fileName = "model-catalog.json"

type Catalog struct {
	UpdatedAt time.Time `json:"updatedAt"`
	Source    string    `json:"source"`
	Models    []Model   `json:"models"`
	Warnings  []string  `json:"warnings,omitempty"`
}

type Model struct {
	ID                     string   `json:"id"`
	ModelKey               string   `json:"modelKey"`
	DisplayName            string   `json:"displayName"`
	Type                   string   `json:"type"`
	Downloaded             bool     `json:"downloaded"`
	Loaded                 bool     `json:"loaded"`
	Architecture           string   `json:"architecture,omitempty"`
	Params                 string   `json:"params,omitempty"`
	SizeBytes              int64    `json:"sizeBytes,omitempty"`
	MaxContextLength       int      `json:"maxContextLength,omitempty"`
	TrainedForToolUse      bool     `json:"trainedForToolUse"`
	Vision                 bool     `json:"vision"`
	EstimatedGPUMemoryGB   float64  `json:"estimatedGpuMemoryGB,omitempty"`
	EstimatedTotalMemoryGB float64  `json:"estimatedTotalMemoryGB,omitempty"`
	HardwareFit            string   `json:"hardwareFit"`
	SpeedClass             string   `json:"speedClass"`
	ComplexityClass        string   `json:"complexityClass"`
	BestFor                []string `json:"bestFor"`
	AvoidFor               []string `json:"avoidFor"`
	Notes                  string   `json:"notes,omitempty"`
}

func DefaultPath() (string, error) {
	dir, err := appconfig.DefaultDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, fileName), nil
}

func Save(path string, catalog Catalog) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create catalog directory: %w", err)
	}

	sort.SliceStable(catalog.Models, func(i, j int) bool {
		return catalog.Models[i].ID < catalog.Models[j].ID
	})

	contents, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("encode model catalog: %w", err)
	}
	contents = append(contents, '\n')

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("write model catalog: %w", err)
	}

	return nil
}

func Load(path string) (Catalog, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Catalog{}, err
		}
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("read model catalog: %w", err)
	}

	var catalog Catalog
	if err := json.Unmarshal(contents, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode model catalog: %w", err)
	}
	if catalog.Source == "" {
		return Catalog{}, errors.New("decode model catalog: source is required")
	}

	return catalog, nil
}

func FindModel(catalog Catalog, query string) (Model, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return Model{}, false
	}

	for _, model := range catalog.Models {
		candidates := []string{
			model.ID,
			model.ModelKey,
			model.DisplayName,
		}
		for _, candidate := range candidates {
			if strings.ToLower(candidate) == needle {
				return model, true
			}
		}
	}

	for _, model := range catalog.Models {
		if strings.Contains(strings.ToLower(model.ID), needle) ||
			strings.Contains(strings.ToLower(model.ModelKey), needle) ||
			strings.Contains(strings.ToLower(model.DisplayName), needle) {
			return model, true
		}
	}

	return Model{}, false
}
