# Pending Commits

## Commit 1: `aigaze serve` — Web-based Dashboard

### Files:
- [x] `internal/web/server.go` — HTTP server + SSE endpoints + action detail builder
- [x] `internal/web/static/index.html` — Dashboard page
- [x] `internal/web/static/style.css` — Dark theme + detail panel + accordion styling
- [x] `internal/web/static/app.js` — SSE client + click-to-expand + auto-scroll
- [x] `cmd/aigaze/main.go` — Add `serve` subcommand (--port, --replay, --speed), bump v0.3.0
- [x] `README.md` — Add serve, replay, keyboard controls, detail expand docs

### Verified:
- `aigaze serve --help` ✅
- `/api/stats` returns `{"sessions":1,"actions":43,"findings":0}` ✅
- `/api/history` returns 43 actions with full detail (filePath, startLine, endLine, command, goal...) ✅
- `/` serves embedded HTML ✅
- SSE `/api/events` endpoint live ✅
- Click-to-expand: detail fields render per tool type (read_file → File/Lines, run_in_terminal → Command/Goal, replace → Old/New) ✅

## Commit 2: Detail expand (TUI + Web)

### Files:
- [x] `internal/tui/tui.go` — Cursor navigation, Enter-to-expand, detail rendering, formatDetail()
- [x] `internal/web/server.go` — buildActionDetail() per-tool argument extraction
- [x] `internal/web/static/app.js` — renderDetail(), renderFindingDetail(), accordion toggle
- [x] `internal/web/static/style.css` — .detail-panel, .detail-old, .detail-new, .detail-cmd, .expand-icon

### TUI keyboard:
- `j/k` moves cursor (with highlight)
- `Enter` toggles detail expand on cursor row
- `▸` / `▾` expand indicators
- Detail shows per-tool: File/Lines, Command/Goal, Old/New, Query/Pattern

### Status: ✅ Ready to commit (awaiting approval)

## Commit 3: Search & Filter (TUI + Web)

### Files:
- [x] `internal/tui/tui.go` — `/` search mode, `n`/`N` next/prev match, `Esc` clear, yellow match highlighting, search bar in stats
- [x] `internal/web/static/index.html` — Filter bar (search input, tool dropdown, severity dropdown, clear button, match count)
- [x] `internal/web/static/style.css` — `.hidden`, `.search-match`, filter bar styles
- [x] `internal/web/static/app.js` — `applyFilters()`, `registerTool()` dynamic dropdown, data-* attributes, filter-on-add

### TUI search:
- `/` enters search mode with input prompt at bottom of stats bar
- Type to live-filter: matches across tool, target, session (actions) and ruleID, severity, evidence, tool (findings)
- `Enter` confirms and jumps to first match
- `Esc` clears filter
- `n`/`N` jumps to next/previous match (wraps)
- Matched rows highlighted in yellow (bold)

### Web filter:
- Search input filters across all text content
- Tool dropdown auto-populates from observed tools
- Severity dropdown filters findings
- Clear button resets all filters
- Match count displayed

### Status: ✅ Ready to commit (awaiting approval)

## Commit 4: Tab-based Dashboard — URL, Process, File, Tool Access Logs

### Files:
- [x] `internal/web/static/index.html` — 6-tab layout (Live Monitor, URL Access, Process Access, File Access, Tool Access, Overview), stats bar with 7 counters, unified filter bars with Risk dropdown per tab
- [x] `internal/web/static/style.css` — Tab bar, `.access-table`, `.access-filter-bar`, `.risk-critical/high/medium/safe/suspicious`, `.op-read/create/edit/list/search`, `.tool-summary`, removed old `.url-status-*`, `.tool-summary-table`, `.tool-detail-table` styles
- [x] `internal/web/static/app.js` — Full tracking logic for all 4 access tabs:
  - URL Access: `addURL()`, `classifyURLWeb()`, risk filter (was "Status", renamed to "Risk")
  - Process Access: `extractBinary()`, `classifyProcess()`, `trackProcess()` — parses chained cmds, 4-level risk (critical/high/medium/safe)
  - File Access: `classifyFileAccess(path, op)` — op-aware 4-level risk (write .ssh = critical, read .env = medium), `mapFileOp()`, `trackFileAccess()`
  - Tool Access: `classifyToolRisk()` by action type, `trackToolUsage()` — redesigned from summary table to flat access log with Risk column
- [x] `internal/web/server.go` — URL extraction + broadcast, `/api/urls` endpoint
- [x] `internal/engine/urls.go` — `URLRecord`, `ExtractURLs()`, `classifyURL()`

### Risk classification matrix:
- **URL**: Suspicious (bad TLD, exfil domain, IP-based, non-HTTPS) / Safe
- **Process**: Critical (nc, nmap, msfvenom…) / High (curl|sh, python -c, base64 -d…) / Medium (curl, unknown binary) / Safe (go, git, ls…)
- **File**: Critical (write to .ssh, .bashrc, /tmp) / High (read keys, write .env, /etc/shadow) / Medium (read .env, .bash_history, /etc/) / Safe
- **Tool**: High (terminal_exec/input) / Medium (web_fetch, file_create/edit) / Safe (file_read, search, list)

### All tabs unified:
- Filter bar: Search + type-specific filters + Risk dropdown + Clear + count
- Summary line below filter bar
- Single access-table with Risk column using shared `.risk-*` CSS classes

### Status: ✅ Ready to commit (awaiting approval)

## Commit 5: Click-to-expand detail on access tabs + alert cross-reference + Playwright E2E tests

### Files:
- [x] `internal/web/static/app.js` — 
  - `attachTableDetail(tr, detail, colSpan)` shared helper for all access tabs
  - `trackToolUsage/trackProcess/trackFileAccess` call `attachTableDetail(tr, a.detail, N)`
  - `addFinding()` builds `alertAction` with detail from finding fields (rule_id, rule_name, severity, evidence, description, mitre)
  - `trackFileAccessFromAlert/trackToolUsageFromAlert/trackProcessFromAlert` now call `attachTableDetail(tr, a.detail, N)` — fixes critical rows not expandable
  - All `apply*Filters()` use `tr:not(.table-detail-row)` to skip detail rows
- [x] `internal/web/static/style.css` — `.table-detail-row` styles (hidden by default, blue left border accent on expand)
- [x] `internal/web/server.go` — `findingJSON` includes Target and ActionType; `relayFindings()` populates them
- [x] `internal/engine/engine.go` — Finding struct includes Target field; all finding creation sites populate Target
- [x] `e2e/package.json` — Playwright test dependencies
- [x] `e2e/playwright.config.ts` — Chromium headless config, baseURL localhost:8080
- [x] `e2e/tests/dashboard.spec.ts` — 16 E2E tests covering:
  - Page load + stats bar
  - All 6 tabs clickable with active class
  - Live Monitor: filter toggle, search, tool/severity/date dropdowns, clear, auto-follow checkboxes, row expand
  - URL Access: search, risk/source dropdowns, clear, row expand
  - Process Access: search, risk dropdown, clear, row expand, expand per risk level
  - File Access: search, op/risk dropdowns, clear, row expand, expand per risk level, all op×risk combos (30)
  - Tool Access: search, tool/risk/action dropdowns, clear, row expand, expand per risk level, tool×risk+action combos
  - Cross-tab filter preservation
  - Rapid tab switching stress test (3 rounds)
  - JS error monitoring (pageerror + console.error)

### Verified: 16/16 tests passed ✅

### Status: ✅ Ready to commit (awaiting approval)

## Commit 6: YAML Rule Engine — R1-R5 externalized + loader

### Files:
- [x] `internal/engine/rules/builtin/R1-credential-file-access.yaml`
- [x] `internal/engine/rules/builtin/R2-dangerous-terminal-command.yaml`
- [x] `internal/engine/rules/builtin/R3-hardcoded-secret.yaml`
- [x] `internal/engine/rules/builtin/R4-workspace-boundary.yaml`
- [x] `internal/engine/rules/builtin/R5-external-url.yaml`
- [x] `internal/engine/loader.go` — YAML parser + embedded loader + external dir loader
- [x] `internal/engine/engine.go` — `ScanSession` uses `LoadAllRules()` with fallback; `SuppressPlaceholders` replaces hardcoded `rule.ID == "R3"`
- [x] `rules/community/.gitkeep` — empty community rules directory
- [x] `docs/community-rules-roadmap.md` — roadmap for community rule contributions
- [x] `go.mod` / `go.sum` — added `gopkg.in/yaml.v3`

### Architecture:
- `rules/builtin/*.yaml` embedded via `//go:embed` — no external files needed
- `LoadBuiltinRules()` — reads from embedded FS
- `LoadExternalRules(dir)` — reads from filesystem directory
- `LoadAllRules(dirs...)` — builtin + external, external overrides by ID
- YAML schema: `id`, `name`, `description`, `severity`, `mitre_technique`, `mitre_name`, `action_types`, `check`, `patterns`, `suppress_placeholders`
- Rule sorting by numeric ID (R1 < R2 < R10)
- Fallback to `DefaultRules()` if YAML loading fails

### Verified:
- `go build` ✅
- `go vet ./...` ✅
- `aigaze scan` produces identical 36 findings ✅

### Status: ✅ Ready to commit (awaiting approval)
