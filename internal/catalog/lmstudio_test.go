package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUpdateFromLMStudioBuildsCatalog(t *testing.T) {
	runner := fakeRunner{
		responses: map[string]fakeResponse{
			"lms ls --json": {
				stdout: `[{"type":"llm","modelKey":"qwen/qwen3-coder-next","displayName":"Qwen3 Coder Next","sizeBytes":48487210160,"paramsString":"80B","architecture":"qwen3next","vision":false,"trainedForToolUse":true,"maxContextLength":262144},{"type":"embedding","modelKey":"text-embedding-nomic-embed-text-v1.5","displayName":"Nomic Embed Text v1.5","sizeBytes":84106624,"architecture":"nomic-bert","maxContextLength":2048}]`,
			},
			"lms ps --json": {
				stdout: `[{"modelKey":"qwen/qwen3-coder-next","identifier":"qwen/qwen3-coder-next","contextLength":4096,"status":"idle"}]`,
			},
			"lms load --estimate-only qwen/qwen3-coder-next": {
				stdout: "Model: qwen/qwen3-coder-next\nEstimated GPU Memory:   48.00 GiB\nEstimated Total Memory: 48.00 GiB\n\nEstimate: This model may be loaded based on your resource guardrails settings.",
			},
		},
	}

	catalog, err := UpdateFromLMStudio(context.Background(), UpdateOptions{
		LMSPath:          "lms",
		IncludeEstimates: true,
		Runner:           runner,
		Now:              func() time.Time { return time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("update catalog: %v", err)
	}

	if len(catalog.Models) != 2 {
		t.Fatalf("model count = %d", len(catalog.Models))
	}
	qwen, ok := FindModel(catalog, "qwen")
	if !ok {
		t.Fatal("qwen model not found")
	}
	if !qwen.Loaded {
		t.Fatal("qwen was not marked loaded")
	}
	if qwen.HardwareFit != "fits" {
		t.Fatalf("hardware fit = %q", qwen.HardwareFit)
	}
	if qwen.SpeedClass != "slow" || qwen.ComplexityClass != "high" {
		t.Fatalf("unexpected qwen score: %+v", qwen)
	}
	embedding, ok := FindModel(catalog, "nomic")
	if !ok {
		t.Fatal("embedding model not found")
	}
	if embedding.Type != "embedding" || embedding.AvoidFor[0] != "chat responses" {
		t.Fatalf("unexpected embedding score: %+v", embedding)
	}
}

func TestUpdateFromLMStudioKeepsWarningsForPSFailure(t *testing.T) {
	runner := fakeRunner{
		responses: map[string]fakeResponse{
			"lms ls --json": {
				stdout: `[{"type":"llm","modelKey":"model-a","displayName":"Model A"}]`,
			},
			"lms ps --json": {
				stderr: "server unavailable",
				err:    errors.New("exit 1"),
			},
		},
	}

	catalog, err := UpdateFromLMStudio(context.Background(), UpdateOptions{
		LMSPath: "lms",
		Runner:  runner,
	})
	if err != nil {
		t.Fatalf("update catalog: %v", err)
	}
	if len(catalog.Warnings) != 1 || !strings.Contains(catalog.Warnings[0], "server unavailable") {
		t.Fatalf("warnings = %+v", catalog.Warnings)
	}
}

func TestParseEstimate(t *testing.T) {
	estimate := parseEstimate("Estimated GPU Memory: 9.00 GiB\nEstimated Total Memory: 11.25 GiB\nEstimate: This model may be loaded based on your resource guardrails settings.")
	if estimate.GPUGB != 9 || estimate.TotalGB != 11.25 || estimate.HardwareFit != "fits" {
		t.Fatalf("estimate = %+v", estimate)
	}
}

type fakeRunner struct {
	responses map[string]fakeResponse
}

type fakeResponse struct {
	stdout string
	stderr string
	err    error
}

func (r fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	key := name + " " + strings.Join(args, " ")
	response, ok := r.responses[key]
	if !ok {
		return nil, []byte("missing fake response for " + key), errors.New("missing fake response")
	}
	return []byte(response.stdout), []byte(response.stderr), response.err
}
