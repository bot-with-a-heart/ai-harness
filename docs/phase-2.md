# Phase 2 LM Studio Provider

Phase 2 adds local model support through LM Studio.

## Phase 2A - Provider Calls

Status: complete.

This subphase adds an OpenAI-compatible LM Studio provider for listing currently served models and sending local chat prompts.

## Commands

```powershell
.\ai-harness.exe models list
.\ai-harness.exe ask-local "Explain what this repository does"
```

Useful options:

```powershell
.\ai-harness.exe models list --provider desktop --timeout 10s
.\ai-harness.exe ask-local --model <model-id> "Explain what this repository does"
```

`ask-local` uses `--model` when supplied. Otherwise, it discovers models from LM Studio and uses the first returned model.

## Phase 2B - Model Catalog Memory

Status: complete.

### Objective

Build a manually refreshed local model catalog so the harness understands which LM Studio models are downloaded, which are loaded, which can run effectively on the machine, and which task types each model is best suited for.

This catalog should be refreshed only when the user asks for it. Normal harness runs should read the cached catalog instead of scanning LM Studio every time.

### Commands

```powershell
.\ai-harness.exe models catalog update
.\ai-harness.exe models catalog show
.\ai-harness.exe models catalog explain <model-id>
```

Useful options:

```powershell
.\ai-harness.exe models catalog update --skip-estimates
.\ai-harness.exe models catalog update --timeout 3m
.\ai-harness.exe models catalog update --lms-path C:\Users\jedid\.lmstudio\bin\lms.exe
.\ai-harness.exe models catalog show --json
.\ai-harness.exe models catalog show --catalog <path>
```

Future routing can use:

```powershell
.\ai-harness.exe models recommend "Refactor this package"
```

### Data Sources

Use LM Studio sources in this order:

```text
lms ls --json --detailed
lms ps
lms load --estimate-only <model-key>
GET /api/v1/models
```

`lms ls --json --detailed` is the best source for downloaded model metadata such as model key, size, architecture, parameters, context length, vision support, and tool-use training.

`lms ps` or SDK equivalents identify models currently loaded into memory.

`lms load --estimate-only <model-key>` should be used to estimate whether a downloaded model fits the current machine/resource guardrails without actually loading it.

`GET /api/v1/models` can provide REST-visible models from the running LM Studio server.

### Catalog Location

Store the generated catalog separately from user-authored config:

```text
~/.ai-harness/model-catalog.json
```

Commands read this file unless the user explicitly runs `models catalog update`.

The catalog should include:

```json
{
  "updatedAt": "...",
  "source": "lmstudio",
  "models": [
    {
      "id": "qwen/qwen3-coder-next",
      "modelKey": "...",
      "displayName": "...",
      "type": "llm",
      "downloaded": true,
      "loaded": false,
      "architecture": "qwen",
      "params": "14B",
      "sizeBytes": 0,
      "maxContextLength": 0,
      "trainedForToolUse": false,
      "vision": false,
      "estimatedGpuMemoryGB": 0,
      "estimatedTotalMemoryGB": 0,
      "hardwareFit": "fits|borderline|too_large|unknown",
      "speedClass": "fast|balanced|slow|unknown",
      "complexityClass": "low|medium|high|unknown",
      "bestFor": ["explanation", "small edits"],
      "avoidFor": ["large refactors"],
      "notes": ""
    }
  ]
}
```

### Capability Scoring

The first implementation should use deterministic heuristics:

```text
small models: fast, low complexity
mid-size coder/instruct models: balanced, medium complexity
large coder/reasoning models: slower, high complexity
embedding models: retrieval/context support only, not chat/editing
vision models: image-aware tasks
tool-trained models: tool/function-call-friendly tasks
hardware estimate too high: avoid unless explicitly requested
```

Later phases can add measured benchmark results, user overrides, and task-outcome history.

### Manual Testing

Verify:

```powershell
.\ai-harness.exe models catalog update
.\ai-harness.exe models catalog show
.\ai-harness.exe models catalog explain <model-id>
```

Expected:

```text
Downloaded models are discovered
Loaded models are marked
Hardware fit estimates are cached
Model suitability summaries are generated
Subsequent commands read the cache without refreshing
```

Completed before Phase 3.

## Phase 2B Review Gate

Stop after build, catalog update, catalog show, model explanation, cache-only behavior review, and architecture review before starting Phase 3.
