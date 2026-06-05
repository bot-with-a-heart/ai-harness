# AI Harness

AI Harness is a local-first AI coding harness planned to route software development tasks between local models, Codex, and future providers.

This repository is currently at Phase 8: escalation workflow.

## Current Commands

```powershell
.\ai-harness.exe version
.\ai-harness.exe help
.\ai-harness.exe config init
.\ai-harness.exe config show
.\ai-harness.exe config doctor
.\ai-harness.exe context
.\ai-harness.exe classify "Refactor my AWS CDK application"
.\ai-harness.exe run "Add tests for the authentication middleware"
.\ai-harness.exe run --edit "Add unit tests"
.\ai-harness.exe run --local-first "Fix failing tests"
.\ai-harness.exe models list
.\ai-harness.exe ask-local "Explain what this repository does"
.\ai-harness.exe ask-codex "Review this repository"
.\ai-harness.exe models catalog update
.\ai-harness.exe models catalog show
.\ai-harness.exe models catalog explain <model-id>
```
