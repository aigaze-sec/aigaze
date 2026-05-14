# AIGaze E2E Tests

Playwright-based UI tests that simulate human interaction with the AIGaze Web Dashboard.
Every button, dropdown, search input, and expandable row is exercised.

## Prerequisites

```bash
cd e2e
npm install
npx playwright install chromium
```

## Running

```bash
# Full suite (headless)
npx playwright test

# Headed mode (see the browser)
npx playwright test --headed

# Single test
npx playwright test --grep "Live Monitor"

# Debug mode (step through)
npx playwright test --debug
```

> **Note**: The AIGaze server must be running on `http://localhost:8080` before tests start.
> ```bash
> cd .. && ./aigaze serve --replay /path/to/transcript.jsonl &
> ```

## Test Cases (16)

| # | Test | What it clicks / checks |
|---|------|------------------------|
| 1 | Page loads with title and stats | Title = "AIGaze Dashboard", `#stat-actions` > 0 |
| 2 | No JS errors on initial load | `pageerror` + `console.error` listeners empty |
| 3 | All 6 tabs clickable and show content | Click each `[data-tab]` button → verify `.active` class + content panel visible |
| 4 | Live Monitor: filter toggle, all controls, clear | `#filter-toggle` → panel visible; `#filter-search` type+clear; `#filter-tool` cycle all options; `#filter-severity` cycle CRITICAL/HIGH/MEDIUM/LOW; `#filter-date-from` + `#filter-date-to` set values; `#filter-clear`; `#action-follow` + `#finding-follow` uncheck/check; click action row + finding row |
| 5 | URL Access: all filters, row expand | `#url-filter-search` type "github"; `#url-filter-risk` cycle suspicious/safe; `#url-filter-source` cycle fetch_webpage/terminal_command; `#url-filter-clear`; click first `.expandable` row → verify `.expanded` + `.table-detail-row` visible → click again to collapse |
| 6 | Process Access: all filters, row expand | `#proc-filter-search` type "python"; `#proc-filter-risk` cycle all; `#proc-filter-clear`; expand + collapse first row |
| 7 | Process Access: expand works per risk filter | For each risk (critical/high/medium/safe): select filter → click first visible expandable row → verify detail shows → collapse |
| 8 | File Access: all filters, row expand | `#file-filter-search` type ".env"; `#file-filter-op` cycle read/create/edit/list/search; `#file-filter-risk` cycle all; `#file-filter-clear`; expand + collapse |
| 9 | File Access: expand works per risk filter | Same per-risk expand test as Process |
| 10 | File Access: cycle all operation × risk combos | 6 ops × 5 risks = **30 combinations**, verify no crash |
| 11 | Tool Access: all filters, row expand | `#tool-filter-search` type "read_file"; `#tool-filter-name` cycle all dynamic options; `#tool-filter-risk` cycle all; `#tool-filter-action` cycle all; `#tool-filter-clear`; expand + collapse |
| 12 | Tool Access: expand works per risk filter | Per-risk expand test |
| 13 | Tool Access: cycle tool × risk + action combos | First 5 tools × 5 risks + 6 action types, verify no crash |
| 14 | Switching tabs preserves filter state | Set Process risk=high → switch to File → switch back → verify filter still "high" |
| 15 | Rapid tab switching does not crash | 3 rounds cycling all 6 tabs at 50ms intervals |
| 16 | Stats bar shows correct counters | All 7 stat elements (`stat-sessions/actions/findings/urls/procs/files/tools`) ≥ 0, actions > 0 |

## Coverage Summary

| Area | Controls tested |
|------|----------------|
| **Tabs** | 6 tab buttons (data-tab click + active class + content visibility) |
| **Live Monitor** | filter-toggle, filter-search, filter-tool, filter-severity, filter-date-from, filter-date-to, filter-clear, action-follow, finding-follow, action row click, finding row click |
| **URL Access** | url-filter-search, url-filter-risk (3 options), url-filter-source (3 options), url-filter-clear, row expand/collapse |
| **Process Access** | proc-filter-search, proc-filter-risk (5 options), proc-filter-clear, row expand/collapse, per-risk expand |
| **File Access** | file-filter-search, file-filter-op (6 options), file-filter-risk (5 options), file-filter-clear, row expand/collapse, per-risk expand, 30 combo tests |
| **Tool Access** | tool-filter-search, tool-filter-name (dynamic), tool-filter-risk (5 options), tool-filter-action (12 options), tool-filter-clear, row expand/collapse, per-risk expand, combo tests |
| **Cross-tab** | Filter state preserved across tab switches |
| **Stress** | Rapid tab cycling (18 switches in < 1s) |
| **Error detection** | JS `pageerror` + `console.error` captured on every test |

## Output

```
Running 16 tests using 1 worker
  ✓  1  page loads with title and stats
  ✓  2  no JS errors on initial load
  ✓  3  all 6 tabs are clickable and show content
  ✓  4  Live Monitor: filter toggle, all controls, clear
  ✓  5  URL Access: all filters, row expand
  ...
  16 passed (4.6m)
```

Failed test screenshots are saved to `test-results/`.
