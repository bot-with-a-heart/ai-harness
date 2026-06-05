package catalog

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadAndFindModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-catalog.json")
	original := Catalog{
		UpdatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		Source:    "lmstudio",
		Models: []Model{
			{ID: "b-model", ModelKey: "b-model", DisplayName: "B Model", HardwareFit: "fits"},
			{ID: "a-model", ModelKey: "a-model", DisplayName: "A Model", HardwareFit: "fits"},
		},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if loaded.Models[0].ID != "a-model" {
		t.Fatalf("models were not sorted before save: %+v", loaded.Models)
	}

	model, ok := FindModel(loaded, "B Model")
	if !ok {
		t.Fatal("model not found by display name")
	}
	if model.ID != "b-model" {
		t.Fatalf("model ID = %q", model.ID)
	}
}
