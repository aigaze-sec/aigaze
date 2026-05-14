# Community Rules Roadmap

Plan for enabling external contributors to write and submit detection rules for AIGaze.

## Current State

- R1–R5 are hardcoded in Go (`internal/engine/engine.go`)
- No external rule loading mechanism
- aigaze-pro (private) already uses YAML ruleset (`rules/full_ruleset.yaml`)

---

## 1. YAML Rule Format (Lowest Barrier)

```yaml
# rules/community/R100-pip-install-untrusted.yaml
id: R100
name: Pip install from untrusted source
severity: HIGH
mitre: T1195.001
match:
  action_type: terminal_exec
  target:
    regex: "pip install.*--index-url(?!.*pypi\\.org)"
description: "AI agent installing packages from non-PyPI source"
```

- No Go code required — PR a single YAML file
- Engine loads all files in `rules/` at startup
- Inspired by Sigma rules, but focused on AI agent actions

## 2. Rule Layer Architecture

| Layer | Who writes | Examples |
|-------|-----------|----------|
| **L0 — Core** | Maintainers | Workspace boundary, dangerous cmd (hardcoded, cannot be disabled) |
| **L1 — Built-in** | Maintainers + reviewed PRs | URL allowlist, credential access patterns |
| **L2 — Community** | Anyone via PR | Framework-specific rules (LangChain, CrewAI, AutoGen) |
| **L3 — Custom** | User local only | `~/.aigaze/rules/` — never enters the repo |

## 3. Contribution Flow

```
1. Issue: "Rule Request: detect XXX pattern"
2. Fork → write YAML rule + test case (minimum 3 JSONL actions)
3. PR template:
   - Rule ID + Name + Severity + MITRE mapping
   - Match conditions
   - Test transcript (1 triggering + 1 non-triggering)
   - False positive assessment
4. CI: automatically runs rule against test fixtures
5. Review → Merge → included in next release
```

## 4. Rule Testing Framework

```bash
# Validate a rule locally after writing it
aigaze rule test rules/community/R100.yaml \
  --should-match fixtures/pip-untrusted.jsonl \
  --should-not-match fixtures/pip-normal.jsonl
```

- Every rule PR must include fixture JSONL files
- CI runs all rules to ensure no conflicts or regressions

## 5. Advanced: Condition DSL

```yaml
match:
  any:
    - action_type: terminal_exec
      target.contains: "curl"
      followed_by:
        action_type: terminal_exec
        target.contains: "| bash"
        within: 3  # within 3 actions
```

Supported operators:
- `contains`, `regex`, `startsWith`, `not`
- `followed_by` / `preceded_by` (sequence detection)
- `within` — time or event window
- `and` / `any` — combinational logic

## 6. Community Ecosystem

- **Rule Registry**: Separate repo `aigaze-rules` for all community rules (like Sigma HQ)
- **Rule Score**: Community rating per rule (false positive rate reporting)
- **Agent-specific packs**: `aigaze-rules-copilot`, `aigaze-rules-cursor`, `aigaze-rules-claude-code`
- **Severity override**: Users can adjust severity or disable rules in `~/.aigaze/config.yaml`

---

## Implementation Plan (Suggested First Steps)

1. **Convert R1–R5 from Go hardcode to YAML + Go engine loader**
   - Keep Go as fallback, load YAML rules on top
   - `internal/engine/loader.go` — parse YAML rule files

2. **Establish `rules/` directory structure + `fixtures/` test data**
   ```
   rules/
     builtin/        # R1-R5 as YAML (shipped with binary)
     community/      # community-contributed rules
   fixtures/
     dangerous-cmd.jsonl
     safe-session.jsonl
   ```

3. **Add `aigaze rule test` subcommand**
   - Validates a rule YAML against fixture files
   - Reports match/no-match results

4. **Add "Contributing Rules" section to README**
   - Link to this roadmap
   - Quick-start guide for writing a rule

5. **Open "good first rule" issues to attract contributors**
   - R11: Detect `npm install` from non-npmjs registry
   - R12: Detect base64-encoded command execution
   - R13: Detect `chmod 777` or `chmod +x` on downloaded files
   - R14: Detect DNS exfiltration patterns in curl/wget
   - R15: Detect AI agent accessing browser cookies/history files
