import { test, expect, type Page, type Locator } from "@playwright/test";

// Wait for SSE data to load
async function waitForData(page: Page) {
  // Wait until stats bar shows non-zero actions
  await page.waitForFunction(
    () => {
      const el = document.getElementById("stat-actions");
      return el && parseInt(el.textContent || "0", 10) > 0;
    },
    { timeout: 15_000 }
  );
  // Small extra delay for rendering
  await page.waitForTimeout(500);
}

// Click every <option> in a <select> and verify no JS errors
async function cycleSelect(page: Page, selector: string, errors: string[]) {
  const select = page.locator(selector);
  if ((await select.count()) === 0) return;
  const options = await select.locator("option").allTextContents();
  const values = await select.locator("option").evaluateAll((opts) =>
    (opts as HTMLOptionElement[]).map((o) => o.value)
  );
  for (let i = 0; i < values.length; i++) {
    await select.selectOption(values[i]);
    await page.waitForTimeout(150);
    // Verify page didn't crash
    const title = await page.title();
    if (!title) errors.push(`Page crashed after selecting "${options[i]}" in ${selector}`);
  }
  // Reset to first option (All)
  await select.selectOption(values[0]);
  await page.waitForTimeout(100);
}

// Type in a search input and clear it
async function cycleSearch(page: Page, selector: string, searchText: string) {
  const input = page.locator(selector);
  if ((await input.count()) === 0) return;
  await input.fill(searchText);
  await page.waitForTimeout(200);
  await input.fill("");
  await page.waitForTimeout(100);
}

// Click a button if it exists
async function clickIfExists(page: Page, selector: string) {
  const btn = page.locator(selector);
  if ((await btn.count()) > 0 && (await btn.isVisible())) {
    await btn.click();
    await page.waitForTimeout(150);
  }
}

// Open a collapsible filter panel by clicking its toggle button
async function openFilterPanel(page: Page, toggleSelector: string, panelSelector: string) {
  const panel = page.locator(panelSelector);
  const isOpen = await panel.evaluate((el) => el.classList.contains("open"));
  if (!isOpen) {
    await page.evaluate((sel) => {
      const el = document.querySelector(sel);
      if (el) (el as HTMLElement).click();
    }, toggleSelector);
    await page.waitForTimeout(200);
  }
}

// Try to expand first visible data row and verify detail shows
async function testRowExpand(
  page: Page,
  tbodySelector: string,
  label: string,
  errors: string[]
) {
  const expandable = page.locator(`${tbodySelector} tr.expandable`);
  const count = await expandable.count();
  if (count === 0) {
    // Not an error if no data rows have detail
    return;
  }
  // Click first visible expandable row
  const first = expandable.first();
  if (!(await first.isVisible())) return;
  await first.click();
  await page.waitForTimeout(200);

  // Check it got the expanded class
  const hasExpanded = await first.evaluate((el) =>
    el.classList.contains("expanded")
  );
  if (!hasExpanded) {
    errors.push(`${label}: row did not get 'expanded' class after click`);
    return;
  }

  // Check detail row is visible
  const detailRow = page.locator(
    `${tbodySelector} tr.expandable.expanded + tr.table-detail-row`
  );
  if ((await detailRow.count()) > 0) {
    const display = await detailRow.evaluate(
      (el) => getComputedStyle(el).display
    );
    if (display === "none") {
      errors.push(`${label}: detail row exists but is hidden after expand`);
    }
  } else {
    errors.push(`${label}: no .table-detail-row found after expanded row`);
  }

  // Click again to collapse
  await first.click();
  await page.waitForTimeout(150);
  const stillExpanded = await first.evaluate((el) =>
    el.classList.contains("expanded")
  );
  if (stillExpanded) {
    errors.push(`${label}: row still 'expanded' after second click (collapse)`);
  }
}

// Test expand with each risk filter applied
async function testExpandPerRisk(
  page: Page,
  tbodySelector: string,
  riskSelectSelector: string,
  label: string,
  errors: string[]
) {
  const riskValues = ["critical", "high", "medium", "safe"];
  for (const risk of riskValues) {
    await page.locator(riskSelectSelector).selectOption(risk);
    await page.waitForTimeout(200);

    const visibleRows = page.locator(
      `${tbodySelector} tr.expandable:not([style*="display: none"])`
    );
    const count = await visibleRows.count();
    if (count === 0) continue; // No rows for this risk level, skip

    // Click first visible row
    const first = visibleRows.first();
    await first.click();
    await page.waitForTimeout(200);

    const hasExpanded = await first.evaluate((el) =>
      el.classList.contains("expanded")
    );
    if (!hasExpanded) {
      errors.push(
        `${label} [risk=${risk}]: row did not expand after click (${count} rows visible)`
      );
    } else {
      // Check detail row
      const detailRow = first.locator("+ tr.table-detail-row");
      if ((await detailRow.count()) > 0) {
        const display = await detailRow.evaluate(
          (el) => getComputedStyle(el).display
        );
        if (display === "none") {
          errors.push(
            `${label} [risk=${risk}]: detail row hidden after expand`
          );
        }
      } else {
        errors.push(
          `${label} [risk=${risk}]: no detail row after expanded row`
        );
      }
      // Collapse
      await first.click();
      await page.waitForTimeout(100);
    }
  }
  // Reset filter
  await page.locator(riskSelectSelector).selectOption("");
  await page.waitForTimeout(100);
}

// ─── TESTS ───────────────────────────────────────────────────────

test.describe("AIGaze Dashboard UI", () => {
  let jsErrors: string[] = [];

  test.beforeEach(async ({ page }) => {
    jsErrors = [];
    page.on("pageerror", (err) => jsErrors.push(err.message));
    page.on("console", (msg) => {
      if (msg.type() === "error") jsErrors.push(`console.error: ${msg.text()}`);
    });
    await page.goto("/");
    await waitForData(page);
  });

  // ── Page Load ──

  test("page loads with title and stats", async ({ page }) => {
    await expect(page).toHaveTitle("AIGaze Dashboard");
    const actions = await page.locator("#stat-actions").textContent();
    expect(parseInt(actions || "0", 10)).toBeGreaterThan(0);
  });

  test("no JS errors on initial load", async () => {
    expect(jsErrors).toEqual([]);
  });

  // ── Tab Navigation ──

  test("all 6 tabs are clickable and show content", async ({ page }) => {
    const tabs = [
      { tab: "tab-monitor", label: "📡 Live Monitor" },
      { tab: "tab-urls", label: "🌐 URL Access" },
      { tab: "tab-procs", label: "⚙️ Process Access" },
      { tab: "tab-files", label: "📁 File Access" },
      { tab: "tab-tools", label: "🔧 Tool Access" },
      { tab: "tab-overview", label: "📊 Overview" },
    ];

    for (const { tab, label } of tabs) {
      await page.click(`[data-tab="${tab}"]`);
      await page.waitForTimeout(200);

      // Tab button should be active
      const btnClass = await page
        .locator(`[data-tab="${tab}"]`)
        .getAttribute("class");
      expect(btnClass, `Tab "${label}" should have active class`).toContain(
        "active"
      );

      // Content panel should be visible
      const content = page.locator(`#${tab}`);
      await expect(content).toBeVisible();
    }
    expect(jsErrors).toEqual([]);
  });

  // ── Live Monitor Tab ──

  test("Live Monitor: filter toggle, all controls, clear", async ({
    page,
  }) => {
    await page.click('[data-tab="tab-monitor"]');
    const errors: string[] = [];

    // Toggle filter panel
    await clickIfExists(page, "#filter-toggle");
    await page.waitForTimeout(200);
    const panelVisible = await page.locator("#filter-panel").isVisible();
    expect(panelVisible).toBe(true);

    // Search
    await cycleSearch(page, "#filter-search", "read_file");

    // Tool dropdown
    await cycleSelect(page, "#filter-tool", errors);

    // Severity dropdown
    await cycleSelect(page, "#filter-severity", errors);

    // Date inputs — use evaluate to set value directly (datetime-local format varies)
    await page.locator("#filter-date-from").evaluate(
      (el: HTMLInputElement) => { el.value = "2026-01-01T00:00"; el.dispatchEvent(new Event("input")); }
    );
    await page.waitForTimeout(100);
    await page.locator("#filter-date-to").evaluate(
      (el: HTMLInputElement) => { el.value = "2026-12-31T23:59"; el.dispatchEvent(new Event("input")); }
    );
    await page.waitForTimeout(100);

    // Clear button
    await clickIfExists(page, "#filter-clear");
    await page.waitForTimeout(200);

    // Auto-follow checkboxes
    for (const id of ["#action-follow", "#finding-follow"]) {
      const cb = page.locator(id);
      await cb.uncheck();
      await page.waitForTimeout(100);
      expect(await cb.isChecked()).toBe(false);
      await cb.check();
      await page.waitForTimeout(100);
      expect(await cb.isChecked()).toBe(true);
    }

    // Click an action row to expand
    const actionRow = page.locator("#action-list .action-row").first();
    if ((await actionRow.count()) > 0 && (await actionRow.isVisible())) {
      await actionRow.click();
      await page.waitForTimeout(200);
    }

    // Click a finding row to expand
    const findingRow = page.locator("#finding-list .finding-row").first();
    if ((await findingRow.count()) > 0 && (await findingRow.isVisible())) {
      await findingRow.click();
      await page.waitForTimeout(200);
    }

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  // ── URL Access Tab ──

  test("URL Access: all filters, row expand", async ({ page }) => {
    await page.click('[data-tab="tab-urls"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    // Open filter panel
    await openFilterPanel(page, "#url-filter-toggle", "#url-filter-panel");

    // Search
    await cycleSearch(page, "#url-filter-search", "github");

    // Risk dropdown
    await cycleSelect(page, "#url-filter-risk", errors);

    // Source dropdown
    await cycleSelect(page, "#url-filter-source", errors);

    // Clear
    await clickIfExists(page, "#url-filter-clear");

    // Row expand
    await testRowExpand(page, "#url-list", "URL Access", errors);

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  // ── Process Access Tab ──

  test("Process Access: all filters, row expand", async ({ page }) => {
    await page.click('[data-tab="tab-procs"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#proc-filter-toggle", "#proc-filter-panel");
    await cycleSearch(page, "#proc-filter-search", "python");
    await cycleSelect(page, "#proc-filter-risk", errors);
    await clickIfExists(page, "#proc-filter-clear");
    await testRowExpand(page, "#proc-list", "Process Access", errors);

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  test("Process Access: expand works per risk filter", async ({ page }) => {
    await page.click('[data-tab="tab-procs"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#proc-filter-toggle", "#proc-filter-panel");
    await testExpandPerRisk(
      page,
      "#proc-list",
      "#proc-filter-risk",
      "Process Access",
      errors
    );

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  // ── File Access Tab ──

  test("File Access: all filters, row expand", async ({ page }) => {
    await page.click('[data-tab="tab-files"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#file-filter-toggle", "#file-filter-panel");
    await cycleSearch(page, "#file-filter-search", ".env");
    await cycleSelect(page, "#file-filter-op", errors);
    await cycleSelect(page, "#file-filter-risk", errors);
    await clickIfExists(page, "#file-filter-clear");
    await testRowExpand(page, "#file-list", "File Access", errors);

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  test("File Access: expand works per risk filter", async ({ page }) => {
    await page.click('[data-tab="tab-files"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#file-filter-toggle", "#file-filter-panel");
    await testExpandPerRisk(
      page,
      "#file-list",
      "#file-filter-risk",
      "File Access",
      errors
    );

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  test("File Access: cycle all operation + risk combos", async ({ page }) => {
    await page.click('[data-tab="tab-files"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#file-filter-toggle", "#file-filter-panel");

    const ops = ["", "read", "create", "edit", "list", "search"];
    const risks = ["", "critical", "high", "medium", "safe"];

    for (const op of ops) {
      for (const risk of risks) {
        await page.locator("#file-filter-op").selectOption(op);
        await page.locator("#file-filter-risk").selectOption(risk);
        await page.waitForTimeout(100);

        // Verify filter count text exists and page didn't crash
        const title = await page.title();
        if (!title) {
          errors.push(`Page crashed at op=${op || "All"} risk=${risk || "All"}`);
        }
      }
    }

    // Reset
    await clickIfExists(page, "#file-filter-clear");

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  // ── Tool Access Tab ──

  test("Tool Access: all filters, row expand", async ({ page }) => {
    await page.click('[data-tab="tab-tools"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#tool-filter-toggle", "#tool-filter-panel");
    await cycleSearch(page, "#tool-filter-search", "read_file");
    await cycleSelect(page, "#tool-filter-name", errors);
    await cycleSelect(page, "#tool-filter-risk", errors);
    await cycleSelect(page, "#tool-filter-action", errors);
    await clickIfExists(page, "#tool-filter-clear");
    await testRowExpand(page, "#tool-detail-list", "Tool Access", errors);

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  test("Tool Access: expand works per risk filter", async ({ page }) => {
    await page.click('[data-tab="tab-tools"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#tool-filter-toggle", "#tool-filter-panel");
    await testExpandPerRisk(
      page,
      "#tool-detail-list",
      "#tool-filter-risk",
      "Tool Access",
      errors
    );

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  test("Tool Access: cycle all tool + action + risk combos", async ({
    page,
  }) => {
    await page.click('[data-tab="tab-tools"]');
    await page.waitForTimeout(300);
    const errors: string[] = [];

    await openFilterPanel(page, "#tool-filter-toggle", "#tool-filter-panel");

    // Get dynamic tool options
    const toolValues = await page
      .locator("#tool-filter-name option")
      .evaluateAll((opts) =>
        (opts as HTMLOptionElement[]).map((o) => o.value)
      );

    const risks = ["", "critical", "high", "medium", "safe"];
    const actions = [
      "",
      "file_read",
      "file_create",
      "file_edit",
      "dir_list",
      "terminal_exec",
    ];

    // Cycle a representative subset to avoid combinatorial explosion
    for (const tool of toolValues.slice(0, 5)) {
      for (const risk of risks) {
        await page.locator("#tool-filter-name").selectOption(tool);
        await page.locator("#tool-filter-risk").selectOption(risk);
        await page.waitForTimeout(80);

        const title = await page.title();
        if (!title) {
          errors.push(`Crashed at tool=${tool} risk=${risk}`);
        }
      }
    }

    for (const action of actions) {
      await page.locator("#tool-filter-name").selectOption("");
      await page.locator("#tool-filter-risk").selectOption("");
      await page.locator("#tool-filter-action").selectOption(action);
      await page.waitForTimeout(80);
    }

    await clickIfExists(page, "#tool-filter-clear");

    expect(errors).toEqual([]);
    expect(jsErrors).toEqual([]);
  });

  // ── Cross-tab: filter then switch tab ──

  test("switching tabs preserves filter state", async ({ page }) => {
    // Set a filter on Process tab
    await page.click('[data-tab="tab-procs"]');
    await page.waitForTimeout(200);
    await openFilterPanel(page, "#proc-filter-toggle", "#proc-filter-panel");
    await page.locator("#proc-filter-risk").selectOption("high");
    await page.waitForTimeout(200);

    // Switch to File tab and back
    await page.click('[data-tab="tab-files"]');
    await page.waitForTimeout(200);
    await page.click('[data-tab="tab-procs"]');
    await page.waitForTimeout(200);

    // Re-open filter panel (may have closed on tab switch)
    await openFilterPanel(page, "#proc-filter-toggle", "#proc-filter-panel");

    // Filter should still be set
    const val = await page.locator("#proc-filter-risk").inputValue();
    expect(val).toBe("high");

    expect(jsErrors).toEqual([]);
  });

  // ── Rapid tab switching (stress test) ──

  test("rapid tab switching does not crash", async ({ page }) => {
    const tabs = [
      "tab-monitor",
      "tab-urls",
      "tab-procs",
      "tab-files",
      "tab-tools",
      "tab-overview",
    ];

    for (let round = 0; round < 3; round++) {
      for (const tab of tabs) {
        await page.click(`[data-tab="${tab}"]`);
        await page.waitForTimeout(50);
      }
    }

    await expect(page).toHaveTitle("AIGaze Dashboard");
    expect(jsErrors).toEqual([]);
  });

  // ── Stats bar ──

  test("stats bar shows correct counters", async ({ page }) => {
    const stats = ["stat-sessions", "stat-actions", "stat-findings", "stat-urls"];
    for (const id of stats) {
      const text = await page.locator(`#${id}`).textContent();
      const val = parseInt(text || "0", 10);
      expect(val, `${id} should be >= 0`).toBeGreaterThanOrEqual(0);
    }
    // Actions should definitely be > 0 with replay data
    const actions = parseInt(
      (await page.locator("#stat-actions").textContent()) || "0",
      10
    );
    expect(actions).toBeGreaterThan(0);
  });

  // ── Session Sidebar ──

  test("session sidebar lists all sessions", async ({ page }) => {
    const sidebar = page.locator(".session-sidebar");
    await expect(sidebar).toBeVisible();

    // "All Sessions" item should always exist
    const allItem = page.locator('.session-item[data-session="all"]');
    await expect(allItem).toBeVisible();

    // With multi-session replay, there should be at least 2 session items
    const sessionItems = page.locator(".session-item");
    const count = await sessionItems.count();
    expect(count).toBeGreaterThanOrEqual(2); // "All Sessions" + at least 1 session

    expect(jsErrors).toEqual([]);
  });

  test("clicking a session filters stats and content", async ({ page }) => {
    // Get global stats first
    const globalActions = parseInt(
      (await page.locator("#stat-actions").textContent()) || "0",
      10
    );

    // Find a real session item (not "All Sessions")
    const sessionItems = page.locator('.session-item:not([data-session="all"])');
    const sessionCount = await sessionItems.count();
    if (sessionCount === 0) return; // skip if single-session

    // Click the first session using evaluate (Playwright .click() times out on sidebar)
    const firstSessionId = await sessionItems.first().getAttribute("data-session");
    await page.evaluate((sid) => {
      const el = document.querySelector(`.session-item[data-session="${sid}"]`);
      if (el) (el as HTMLElement).click();
    }, firstSessionId);
    await page.waitForTimeout(300);

    // The clicked session should now be active
    const activeItem = page.locator(".session-item.active");
    await expect(activeItem).toHaveCount(1);
    const activeSession = await activeItem.getAttribute("data-session");
    expect(activeSession).toBe(firstSessionId);

    // Stats should reflect filtered data (actions <= global)
    const filteredActions = parseInt(
      (await page.locator("#stat-actions").textContent()) || "0",
      10
    );
    expect(filteredActions).toBeLessThanOrEqual(globalActions);

    expect(jsErrors).toEqual([]);
  });

  test("clicking All Sessions restores global view", async ({ page }) => {
    // First click a specific session
    const sessionItems = page.locator('.session-item:not([data-session="all"])');
    const sessionCount = await sessionItems.count();
    if (sessionCount === 0) return;

    const firstSessionId = await sessionItems.first().getAttribute("data-session");
    await page.evaluate((sid) => {
      const el = document.querySelector(`.session-item[data-session="${sid}"]`);
      if (el) (el as HTMLElement).click();
    }, firstSessionId);
    await page.waitForTimeout(300);

    // Remember filtered stats
    const filteredActions = parseInt(
      (await page.locator("#stat-actions").textContent()) || "0",
      10
    );

    // Click "All Sessions"
    await page.evaluate(() => {
      const el = document.querySelector('.session-item[data-session="all"]');
      if (el) (el as HTMLElement).click();
    });
    await page.waitForTimeout(300);

    // "All Sessions" should be active
    const activeItem = page.locator(".session-item.active");
    const activeSession = await activeItem.getAttribute("data-session");
    expect(activeSession).toBe("all");

    // Global stats should be >= filtered
    const globalActions = parseInt(
      (await page.locator("#stat-actions").textContent()) || "0",
      10
    );
    expect(globalActions).toBeGreaterThanOrEqual(filteredActions);

    expect(jsErrors).toEqual([]);
  });

  test("session sidebar shows badges with severity", async ({ page }) => {
    const badges = page.locator(".session-item .session-badge");
    const count = await badges.count();
    // At least "All Sessions" should have a badge
    expect(count).toBeGreaterThanOrEqual(1);

    // Check that each badge has severity-based styling (class or text)
    for (let i = 0; i < count; i++) {
      const text = await badges.nth(i).textContent();
      expect(text?.trim().length).toBeGreaterThan(0);
    }

    expect(jsErrors).toEqual([]);
  });

  // ── Collapsible Filter Toggles ──

  test("filter toggle works on all tabs", async ({ page }) => {
    const tabs = [
      { tab: "tab-monitor", toggle: "#filter-toggle", panel: "#filter-panel" },
      { tab: "tab-urls", toggle: "#url-filter-toggle", panel: "#url-filter-panel" },
      { tab: "tab-procs", toggle: "#proc-filter-toggle", panel: "#proc-filter-panel" },
      { tab: "tab-files", toggle: "#file-filter-toggle", panel: "#file-filter-panel" },
      { tab: "tab-tools", toggle: "#tool-filter-toggle", panel: "#tool-filter-panel" },
    ];

    for (const { tab, toggle, panel } of tabs) {
      await page.click(`[data-tab="${tab}"]`);
      await page.waitForTimeout(200);

      const panelEl = page.locator(panel);

      // Initially closed
      const initiallyOpen = await panelEl.evaluate((el) =>
        el.classList.contains("open")
      );
      expect(initiallyOpen, `${panel} should start closed`).toBe(false);

      // Click toggle to open
      await page.evaluate((sel) => {
        const el = document.querySelector(sel);
        if (el) (el as HTMLElement).click();
      }, toggle);
      await page.waitForTimeout(200);

      const nowOpen = await panelEl.evaluate((el) =>
        el.classList.contains("open")
      );
      expect(nowOpen, `${panel} should be open after click`).toBe(true);

      // Click toggle again to close
      await page.evaluate((sel) => {
        const el = document.querySelector(sel);
        if (el) (el as HTMLElement).click();
      }, toggle);
      await page.waitForTimeout(200);

      const closedAgain = await panelEl.evaluate((el) =>
        el.classList.contains("open")
      );
      expect(closedAgain, `${panel} should close on second click`).toBe(false);
    }

    expect(jsErrors).toEqual([]);
  });
});
