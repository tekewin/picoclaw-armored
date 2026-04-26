# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

**picoclaw-armored** is a security-hardened fork of PicoClaw — an ultra-lightweight personal AI agent written in Go targeting embedded Linux boards (<16MB RAM, 1-second boot). The "armored" suffix reflects a deliberate security audit pass that fixed 15 critical-to-medium vulnerabilities. Remote communication channels were narrowed to WhatsApp and Discord only.

Module: `github.com/sipeed/picoclaw`

## Build Commands

```bash
make deps            # go mod download && go mod verify
make generate        # copies workspace/ into onboard embed — required on fresh clone
make build           # build for current platform → build/picoclaw-<platform>-<arch>
make build-all       # cross-compile for all supported targets
make build-linux-arm64     # Raspberry Pi 64-bit
make build-whatsapp-native # with -tags whatsapp_native (uses whatsmeow)
make install         # build then install to ~/.local/bin/picoclaw
```

All builds use `CGO_ENABLED=0`. **Run `make generate` before `make build` on a fresh clone** or after editing files under `workspace/` — the onboard command embeds them via `//go:embed`.

## Lint and Test

```bash
make vet             # go vet ./...
make lint            # golangci-lint run
make fmt             # golangci-lint fmt (gci, gofmt, gofumpt, goimports, golines max 120)
make fix             # golangci-lint run --fix
make test            # go test ./...
make check           # deps + fmt + vet + test
```

Run a single test:
```bash
CGO_ENABLED=0 go test -v -run TestName ./pkg/some/package/
```

Integration tests require `-tags integration` (e.g., `pkg/providers/*_integration_test.go`).

## Architecture

PicoClaw is a **multi-channel AI agent gateway** with three execution modes:
- `picoclaw agent` — Interactive CLI or one-shot chat
- `picoclaw gateway` — Long-running daemon with channel integrations, cron, heartbeat, HTTP health server
- `picoclaw-launcher` / `picoclaw-launcher-tui` — Web/TUI config editors

### Core Message Flow (Gateway Mode)

```
Chat Channel (WhatsApp/Discord)
    → MessageBus.inbound (buffered, cap=64)
    → AgentLoop.Run() → processMessage() → resolves route → picks AgentInstance
    → runAgentLoop() → loads session history → builds context
    → runLLMIteration() → LLMProvider.Chat()
    → [if tool calls] ToolRegistry.ExecuteWithContext() (parallel goroutines)
    → loop until no tool calls or max_iterations
    → final response → MessageBus.outbound → Channel.Send() → user
```

### Key Packages

| Package | Purpose |
|---|---|
| `pkg/agent` | Core agent loop, context building, memory, session management, agent registry |
| `pkg/providers` | `LLMProvider` interface, factory, fallback chains, all provider implementations |
| `pkg/tools` | Tool registry and all tool implementations (shell, file, web, MCP, skills, spawn, cron) |
| `pkg/channels` | Channel interface, manager, Discord, WhatsApp (bridge + native whatsmeow) |
| `pkg/bus` | In-process message bus (buffered Go channels: inbound/outbound/outbound-media) |
| `pkg/config` | JSON config with `caarlos0/env` env-var overlay, migration logic |
| `pkg/routing` | 7-level priority cascade: peer > parent_peer > guild > team > account > channel > default |
| `pkg/session` | Per-session conversation history and token usage tracking with async summarization |
| `pkg/memory` | JSONL-backed persistent message store with compaction |
| `pkg/mcp` | MCP server manager (`modelcontextprotocol/go-sdk`) |
| `pkg/skills` | Skill loader (workspace + global + builtin), ClawHub registry |
| `pkg/auth` | OAuth 2.0 with PKCE, token store, Google/Anthropic/OpenAI flows |

### Provider System

`LLMProvider` interface (single method):
```go
Chat(ctx, messages, tools, model string, options map[string]any) (*LLMResponse, error)
```

`CreateProvider(cfg)` in `pkg/providers/factory_provider.go` dispatches by `vendor/model-id` prefix:
- `openai_compat.Provider` — all OpenAI-compatible APIs
- `anthropic.Provider` — native Anthropic API
- `AntigravityProvider` — Google Cloud Code Assist (OAuth, wraps Gemini)
- `ClaudeCLIProvider` / `CodexCLIProvider` — local CLI subprocess wrappers
- `GitHubCopilotProvider` — gRPC via `github/copilot-sdk/go`

`FallbackChain` (`pkg/providers/fallback.go`) rotates providers on retriable errors (auth, rate limit, billing, timeout, overload) — not on format errors.

### Tools

`ToolRegistry.ToProviderDefs()` **always sorts tools alphabetically** — this is intentional for LLM prompt cache stability. Do not break this ordering.

Multiple tool calls from a single LLM response are dispatched concurrently but results are reordered back to original sequence before appending to history.

Security-critical (`pkg/tools/shell.go`):
- `absoluteDenyPatterns` — hardcoded, cannot be disabled via config: `rm -rf`, `sudo`, `eval`, `nc`, `crontab`, command substitution `$()`, `${}`, backticks, pipe-to-shell
- `defaultDenyPatterns` — can be overridden via `custom_allow_patterns` in config

### Skills System

Skills are Markdown files (`SKILL.md`) with optional YAML frontmatter. Injected into system prompt. Search path (priority order):
1. `<workspace>/skills/` — user-defined
2. `~/.picoclaw/skills/` — global
3. `<cwd>/skills/` — builtin (override with `PICOCLAW_BUILTIN_SKILLS`)

Built-in skills are embedded via `//go:embed workspace` in `cmd/picoclaw/internal/onboard/`.

### Configuration

Config file: `~/.picoclaw/config.json` (override with `PICOCLAW_CONFIG`)  
Home dir: `~/.picoclaw` (override with `PICOCLAW_HOME`)

Env var overlay: any config key can be set via env, e.g. `PICOCLAW_HEARTBEAT_ENABLED=false`.

Modern config uses `model_list[]` array with `{model_name, model: "vendor/model-id", api_key, ...}`. Legacy `providers.*` keys are still supported for backward compatibility.

Antigravity (Google Cloud Code Assist) requires `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` env vars.

See `config/config.example.json` for the full template.

## Security Invariants

These must not be regressed:

1. **Exec absolute deny patterns** (`pkg/tools/shell.go`) are hardcoded and bypass-proof by design (audit CRIT-3).
2. **SSRF protection** (`web_fetch`): `rejectSSRFTarget()` resolves DNS before IP range checks to prevent DNS rebinding attacks.
3. **Launcher CSRF** (`cmd/picoclaw-launcher`): `RequireLocalOrigin` middleware rejects all `/api/` and `/auth/` requests with non-localhost `Origin` (audit CRIT-2).
4. **Antigravity credentials** are read from env vars only — never stored in config (audit CRIT-1).

## Logging Convention

Use `logger.InfoCF / WarnCF / ErrorCF / DebugCF` with a component string and `map[string]any` for fields. Never use `fmt.Printf` for operational logs.

## Build Tags

- `-tags stdjson` — applied by default in Makefile
- `-tags whatsapp_native` — enables whatsmeow-based WhatsApp (larger binary)
- `-tags integration` — enables integration tests

Platform-specific files use `_linux.go` / `_other.go` / `_windows.go` suffixes.
