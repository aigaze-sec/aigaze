# AIGaze

**Audit AI agent actions from transcript files.**

AIGaze monitors what your AI coding agents (Copilot, Cursor, Claude Code, Windsurf) actually do — every file read, every terminal command, every URL fetch — and flags security risks in real time.

## Why

Your AI agent has access to your filesystem, terminal, and network. It can read `~/.ssh/id_rsa`, run `curl | sh`, or push hardcoded secrets to GitHub. No existing security tool watches this layer.

AIGaze does.

## Install

```bash
# From source
go install github.com/aigaze-sec/aigaze@latest

# Or download binary from Releases
```

## Quick Start

```bash
# See what transcripts exist on your machine
aigaze discover

# Scan all transcripts for security findings
aigaze scan --auto

# Real-time monitoring (TUI dashboard)
aigaze watch

# Offline replay a specific transcript in TUI
aigaze watch --replay session.jsonl

# Web dashboard
aigaze serve
```

## Commands

### `aigaze discover`

Auto-finds transcript files from all supported AI agents:

```
Found transcripts in 2 location(s):

  [vscode-copilot] ~/.vscode-server/.../transcripts
  13 sessions, 73.7MB

  [cursor] ~/.cursor/transcripts
  3 sessions, 8.2MB
```

### `aigaze scan [path] [--auto]`

Scans transcripts against L1 detection rules:

```
══════════════════════════════════════════════════════════════
  Session: 5467346c-7fb4-4197-afb7-7948256f9e01
  Actions: 5962  |  Turns: 7315
══════════════════════════════════════════════════════════════

  ⚠ 34 finding(s):

  Rule   Severity   Name                           MITRE           Evidence
  ------ ---------- ------------------------------ --------------- --------
  R1     HIGH       Credential File Access         T1552.001       .ssh/id_rsa
  R2     CRITICAL   Dangerous Terminal Command     T1059           curl ... | sh
  R3     CRITICAL   Hardcoded Secret in Code       T1552.001       ghp_abc...
```

Options:
- `--auto` — auto-discover and scan all transcripts
- `--format json` — JSON output
- `--output report.json` — write to file
- `--workspace /path` — workspace root for boundary checks

### `aigaze watch [path]`

Real-time TUI monitoring dashboard with scrollable panels:

```
  AIGaze Watch — Real-time AI Agent Action Monitor
  📡 Sessions: 14   │   Actions: 342   │   Alerts: 3   │   ↑↓/jk: scroll  tab: switch  f: follow

  ▸ Actions  Time     Session  Tool                      Target
             13:42:05 c168902  read_file                 /home/user/.ssh/id_rsa
             13:42:07 c168902  run_in_terminal           git push origin main
             13:42:08 c168902  fetch_webpage             https://example.com
    ↕ 1–25 of 342
```

Without a path, auto-discovers and monitors all AI agent transcripts.

**Offline replay mode** — import any JSONL transcript for post-hoc analysis:

```bash
aigaze watch --replay session.jsonl            # instant replay
aigaze watch --replay session.jsonl --speed 50 # 50ms delay between events
```

**Keyboard controls:**

| Key | Action |
|-----|--------|
| `j`/`k` or `↑`/`↓` | Scroll up/down |
| `PgDn`/`PgUp` or `Ctrl+D`/`U` | Half-page jump |
| `g`/`G` or `Home`/`End` | Jump to top/bottom |
| `Tab` | Switch focus: Actions ↔ Alerts |
| `f` | Toggle auto-follow |
| `c` | Clear screen |
| `q` | Quit |

### `aigaze serve [path]`

Web-based dashboard with real-time SSE streaming:

```bash
aigaze serve                          # auto-discover, default port 8080
aigaze serve --port 3000              # custom port
aigaze serve --replay session.jsonl   # offline replay in browser
```

Opens a dark-themed dashboard at `http://localhost:8080` with:

- **Split panels** — Actions stream (left) + Alerts (right)
- **Auto-follow** — scrolls to bottom on new data, pauses when you scroll up
- **SSE (Server-Sent Events)** — real-time push, no polling
- **Late-join support** — new browser tabs load full history
- **Embedded static files** — no external dependencies, single binary

## Detection Rules

| Rule | Severity | What it detects |
|------|----------|----------------|
| R1 | HIGH | Credential/sensitive file access (`.ssh`, `.aws`, `.env`, etc.) |
| R2 | CRITICAL | Dangerous terminal commands (`rm -rf /`, `curl\|sh`, reverse shells) |
| R3 | CRITICAL | Hardcoded secrets in code (AWS keys, GitHub PATs, API keys) |
| R4 | MEDIUM | Out-of-workspace file access |
| R5 | LOW | External URL access |

## Supported AI Agents

| Agent | Status |
|-------|--------|
| VS Code Copilot Chat | ✅ |
| Cursor | ✅ |
| Claude Code (CLI) | ✅ |
| Windsurf | ✅ |
| VS Code Insiders | ✅ |

Works on **Linux**, **macOS**, **Windows**, and **WSL**.

## Build

```bash
git clone https://github.com/aigaze-sec/aigaze.git
cd aigaze
go build -o aigaze ./cmd/aigaze/
```

Cross-compile:

```bash
GOOS=darwin  GOARCH=arm64 go build -o aigaze-darwin-arm64  ./cmd/aigaze/
GOOS=linux   GOARCH=amd64 go build -o aigaze-linux-amd64   ./cmd/aigaze/
GOOS=windows GOARCH=amd64 go build -o aigaze-windows.exe   ./cmd/aigaze/
```

## Contributing Rules

Detection rules are YAML files in [`rules/builtin/`](rules/builtin/). Community rules go in [`rules/community/`](rules/community/).

### Rule format

```yaml
id: R100
name: My Custom Rule
description: Detects something suspicious
severity: HIGH          # CRITICAL, HIGH, MEDIUM, LOW
mitre_technique: T1059
mitre_name: Command and Scripting Interpreter
action_types:
  - terminal_exec
check: pattern          # pattern, workspace_boundary, url_allowlist
patterns:
  - "suspicious_command"
  - "another\\.pattern"
```

### Test your rule

Create fixture JSONL files (see `fixtures/` for examples), then:

```bash
aigaze rule test rules/community/R100-my-rule.yaml \
  --should-match fixtures/R100-trigger.jsonl \
  --should-not-match fixtures/R100-safe.jsonl
```

### List all loaded rules

```bash
aigaze rule list
```

### Contribution steps

1. Write a YAML rule in `rules/community/`
2. Create trigger + safe fixture JSONL files in `fixtures/`
3. Run `aigaze rule test` — both assertions must pass
4. Submit a PR

See [`docs/community-rules-roadmap.md`](docs/community-rules-roadmap.md) for the full roadmap and rule layer architecture.

## License

AGPL-3.0
