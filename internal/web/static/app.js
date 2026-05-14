(function () {
  "use strict";

  // DOM refs
  const statSessions = document.getElementById("stat-sessions");
  const statActions = document.getElementById("stat-actions");
  const statFindings = document.getElementById("stat-findings");
  const statUrls = document.getElementById("stat-urls");
  const modeBadge = document.getElementById("mode-badge");
  const actionList = document.getElementById("action-list");
  const findingList = document.getElementById("finding-list");
  const actionFollowCb = document.getElementById("action-follow");
  const findingFollowCb = document.getElementById("finding-follow");
  const filterSearch = document.getElementById("filter-search");
  const filterTool = document.getElementById("filter-tool");
  const filterSeverity = document.getElementById("filter-severity");
  const filterDateFrom = document.getElementById("filter-date-from");
  const filterDateTo = document.getElementById("filter-date-to");
  const filterClear = document.getElementById("filter-clear");
  const filterCount = document.getElementById("filter-count");
  const filterToggle = document.getElementById("filter-toggle");
  const filterPanel = document.getElementById("filter-panel");
  const urlList = document.getElementById("url-list");
  const urlSummaryEl = document.getElementById("url-summary");
  const urlFilterSearch = document.getElementById("url-filter-search");
  const urlFilterRisk = document.getElementById("url-filter-risk");
  const urlFilterSource = document.getElementById("url-filter-source");
  const urlFilterClear = document.getElementById("url-filter-clear");
  const urlFilterCount = document.getElementById("url-filter-count");
  const statTools = document.getElementById("stat-tools");
  const toolDetailList = document.getElementById("tool-detail-list");
  const toolSummaryEl = document.getElementById("tool-summary");
  const toolFilterSearch = document.getElementById("tool-filter-search");
  const toolFilterName = document.getElementById("tool-filter-name");
  const toolFilterAction = document.getElementById("tool-filter-action");
  const toolFilterRisk = document.getElementById("tool-filter-risk");
  const toolFilterClear = document.getElementById("tool-filter-clear");
  const toolFilterCount = document.getElementById("tool-filter-count");
  // Process Access
  const statProcs = document.getElementById("stat-procs");
  const procList = document.getElementById("proc-list");
  const procSummaryEl = document.getElementById("proc-summary");
  const procFilterSearch = document.getElementById("proc-filter-search");
  const procFilterRisk = document.getElementById("proc-filter-risk");
  const procFilterClear = document.getElementById("proc-filter-clear");
  const procFilterCount = document.getElementById("proc-filter-count");
  // File Access
  const statFiles = document.getElementById("stat-files");
  const fileList = document.getElementById("file-list");
  const fileSummaryEl = document.getElementById("file-summary");
  const fileFilterSearch = document.getElementById("file-filter-search");
  const fileFilterOp = document.getElementById("file-filter-op");
  const fileFilterRisk = document.getElementById("file-filter-risk");
  const fileFilterClear = document.getElementById("file-filter-clear");
  const fileFilterCount = document.getElementById("file-filter-count");

  // --- Tab switching ---
  var tabs = document.querySelectorAll(".tab-bar .tab");
  tabs.forEach(function (btn) {
    btn.addEventListener("click", function () {
      tabs.forEach(function (t) { t.classList.remove("active"); });
      document.querySelectorAll(".tab-content").forEach(function (c) { c.classList.remove("active"); });
      btn.classList.add("active");
      document.getElementById(btn.getAttribute("data-tab")).classList.add("active");
    });
  });

  // Toggle filter panel
  filterToggle.addEventListener("click", function () {
    filterPanel.classList.toggle("open");
    filterToggle.classList.toggle("active");
    filterToggle.textContent = filterPanel.classList.contains("open")
      ? "🔍 Filters ▴"
      : "🔍 Filters ▾";
  });

  let actionAutoFollow = true;
  let findingAutoFollow = true;

  // Track known tools for dropdown
  var knownTools = {};

  // --- Filter state ---
  var currentSearch = "";
  var currentTool = "";
  var currentSeverity = "";
  var currentDateFrom = "";
  var currentDateTo = "";

  filterSearch.addEventListener("input", function () {
    currentSearch = this.value.toLowerCase();
    applyFilters();
  });

  filterTool.addEventListener("change", function () {
    currentTool = this.value;
    applyFilters();
  });

  filterSeverity.addEventListener("change", function () {
    currentSeverity = this.value;
    applyFilters();
  });

  filterDateFrom.addEventListener("change", function () {
    currentDateFrom = this.value;
    applyFilters();
  });

  filterDateTo.addEventListener("change", function () {
    currentDateTo = this.value;
    applyFilters();
  });

  filterClear.addEventListener("click", function () {
    filterSearch.value = "";
    filterTool.value = "";
    filterSeverity.value = "";
    filterDateFrom.value = "";
    filterDateTo.value = "";
    currentSearch = "";
    currentTool = "";
    currentSeverity = "";
    currentDateFrom = "";
    currentDateTo = "";
    applyFilters();
  });

  function applyFilters() {
    var fromTs = currentDateFrom ? new Date(currentDateFrom).getTime() : 0;
    var toTs = currentDateTo ? new Date(currentDateTo).getTime() : Infinity;

    var totalActions = 0;
    var visibleActions = 0;
    var rows = actionList.querySelectorAll(".action-row");
    rows.forEach(function (row) {
      totalActions++;
      var tool = row.getAttribute("data-tool") || "";
      var text = (row.getAttribute("data-searchtext") || "").toLowerCase();
      var rowTime = row.getAttribute("data-time") || "";
      var show = true;
      if (currentSearch && text.indexOf(currentSearch) === -1) show = false;
      if (currentTool && tool !== currentTool) show = false;
      if (show && (currentDateFrom || currentDateTo) && rowTime) {
        var ts = new Date(rowTime.replace(" ", "T")).getTime();
        if (ts < fromTs || ts > toTs) show = false;
      }
      if (show) {
        row.classList.remove("hidden");
        visibleActions++;
      } else {
        row.classList.add("hidden");
      }
    });

    var totalFindings = 0;
    var visibleFindings = 0;
    var fRows = findingList.querySelectorAll(".finding-row");
    fRows.forEach(function (row) {
      totalFindings++;
      var sev = row.getAttribute("data-severity") || "";
      var text = (row.getAttribute("data-searchtext") || "").toLowerCase();
      var rowTime = row.getAttribute("data-time") || "";
      var show = true;
      if (currentSearch && text.indexOf(currentSearch) === -1) show = false;
      if (currentSeverity && sev !== currentSeverity) show = false;
      if (show && (currentDateFrom || currentDateTo) && rowTime) {
        var ts = new Date(rowTime.replace(" ", "T")).getTime();
        if (ts < fromTs || ts > toTs) show = false;
      }
      if (show) {
        row.classList.remove("hidden");
        visibleFindings++;
      } else {
        row.classList.add("hidden");
      }
    });

    // Update filter count
    var hasFilter = currentSearch || currentTool || currentSeverity || currentDateFrom || currentDateTo;
    if (hasFilter) {
      filterCount.textContent = "Showing " + visibleActions + "/" + totalActions + " actions, " + visibleFindings + "/" + totalFindings + " alerts";
    } else {
      filterCount.textContent = "";
    }
  }

  function registerTool(toolName) {
    if (!toolName || knownTools[toolName]) return;
    knownTools[toolName] = true;
    var opt = document.createElement("option");
    opt.value = toolName;
    opt.textContent = toolName;
    filterTool.appendChild(opt);
  }

  // --- Auto-follow logic ---
  actionFollowCb.addEventListener("change", function () {
    actionAutoFollow = this.checked;
  });

  findingFollowCb.addEventListener("change", function () {
    findingAutoFollow = this.checked;
  });

  // Pause auto-follow when user scrolls up
  actionList.addEventListener("scroll", function () {
    const atBottom =
      actionList.scrollHeight - actionList.scrollTop - actionList.clientHeight < 30;
    if (!atBottom && actionAutoFollow) {
      actionAutoFollow = false;
      actionFollowCb.checked = false;
    } else if (atBottom && !actionAutoFollow) {
      actionAutoFollow = true;
      actionFollowCb.checked = true;
    }
  });

  findingList.addEventListener("scroll", function () {
    const atBottom =
      findingList.scrollHeight - findingList.scrollTop - findingList.clientHeight < 30;
    if (!atBottom && findingAutoFollow) {
      findingAutoFollow = false;
      findingFollowCb.checked = false;
    } else if (atBottom && !findingAutoFollow) {
      findingAutoFollow = true;
      findingFollowCb.checked = true;
    }
  });

  function scrollToBottom(el) {
    el.scrollTop = el.scrollHeight;
  }

  // --- Detail rendering ---
  function renderDetail(detail) {
    if (!detail || Object.keys(detail).length === 0) return "";
    var rows = [];
    var order = ["filePath", "path", "command", "explanation", "goal",
      "startLine", "endLine", "oldString", "newString", "content",
      "query", "includePattern", "urls", "raw"];
    order.forEach(function (key) {
      if (detail[key] === undefined) return;
      var val = detail[key];
      if (Array.isArray(val)) {
        val = val.join(", ");
      }
      var label = key;
      // Special display for diff-like fields
      var cls = "detail-value";
      if (key === "oldString") { label = "old"; cls = "detail-value detail-old"; }
      if (key === "newString") { label = "new"; cls = "detail-value detail-new"; }
      if (key === "command") { cls = "detail-value detail-cmd"; }
      rows.push(
        '<div class="detail-row">' +
          '<span class="detail-label">' + esc(label) + '</span>' +
          '<span class="' + cls + '">' + esc(String(val)) + '</span>' +
        '</div>'
      );
    });
    // Remaining keys not in order
    Object.keys(detail).forEach(function (key) {
      if (order.indexOf(key) !== -1) return;
      rows.push(
        '<div class="detail-row">' +
          '<span class="detail-label">' + esc(key) + '</span>' +
          '<span class="detail-value">' + esc(String(detail[key])) + '</span>' +
        '</div>'
      );
    });
    return '<div class="detail-panel">' + rows.join("") + '</div>';
  }

  function renderFindingDetail(f) {
    var rows = [];
    if (f.description) {
      rows.push('<div class="detail-row"><span class="detail-label">description</span><span class="detail-value">' + esc(f.description) + '</span></div>');
    }
    if (f.mitre && f.mitre.trim()) {
      rows.push('<div class="detail-row"><span class="detail-label">MITRE</span><span class="detail-value">' + esc(f.mitre) + '</span></div>');
    }
    if (f.tool) {
      rows.push('<div class="detail-row"><span class="detail-label">tool</span><span class="detail-value">' + esc(f.tool) + '</span></div>');
    }
    if (f.evidence) {
      rows.push('<div class="detail-row"><span class="detail-label">evidence</span><span class="detail-value detail-cmd">' + esc(f.evidence) + '</span></div>');
    }
    if (rows.length === 0) return "";
    return '<div class="detail-panel">' + rows.join("") + '</div>';
  }

  function toggleDetail(row) {
    var panel = row.querySelector(".detail-panel");
    if (!panel) return;
    var isOpen = row.classList.contains("expanded");
    if (isOpen) {
      row.classList.remove("expanded");
      panel.style.display = "none";
    } else {
      row.classList.add("expanded");
      panel.style.display = "block";
    }
  }

  // --- Rendering ---
  var MAX_ROWS = 500;

  function addAction(a) {
    registerTool(a.tool);
    var row = document.createElement("div");
    row.className = "action-row";
    row.setAttribute("data-tool", a.tool || "");
    row.setAttribute("data-time", a.time || "");
    row.setAttribute("data-searchtext", [a.tool, a.target, a.session_id, a.time].join(" "));
    var detailHTML = renderDetail(a.detail);
    var expandIcon = detailHTML ? '<span class="expand-icon">▸</span>' : '<span class="expand-icon-spacer"></span>';
    row.innerHTML =
      '<div class="row-summary">' +
        expandIcon +
        '<span class="col-time">' + esc(a.time) + "</span>" +
        '<span class="col-session">' + esc(a.session_id) + "</span>" +
        '<span class="col-tool">' + esc(a.tool) + "</span>" +
        '<span class="col-target" title="' + escAttr(a.target) + '">' + esc(a.target) + "</span>" +
      '</div>' +
      detailHTML;
    if (detailHTML) {
      row.querySelector(".detail-panel").style.display = "none";
      row.querySelector(".row-summary").addEventListener("click", function () {
        toggleDetail(row);
        var icon = row.querySelector(".expand-icon");
        if (icon) icon.textContent = row.classList.contains("expanded") ? "▾" : "▸";
      });
      row.style.cursor = "pointer";
    }

    // Apply current filter to new row
    var hasFilter = currentSearch || currentTool || currentDateFrom || currentDateTo;
    if (hasFilter) {
      var text = row.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (currentSearch && text.indexOf(currentSearch) === -1) show = false;
      if (currentTool && a.tool !== currentTool) show = false;
      if (show && (currentDateFrom || currentDateTo) && a.time) {
        var ts = new Date(a.time.replace(" ", "T")).getTime();
        var fromTs = currentDateFrom ? new Date(currentDateFrom).getTime() : 0;
        var toTs = currentDateTo ? new Date(currentDateTo).getTime() : Infinity;
        if (ts < fromTs || ts > toTs) show = false;
      }
      if (!show) row.classList.add("hidden");
    }

    actionList.appendChild(row);

    // Track tool usage
    trackToolUsage(a);
    trackProcess(a);
    trackFileAccess(a);

    // Overview tracking
    ovTrackAction(a);

    // Trim old rows
    while (actionList.children.length > MAX_ROWS) {
      actionList.removeChild(actionList.firstChild);
    }

    if (actionAutoFollow) {
      scrollToBottom(actionList);
    }
  }

  function addFinding(f) {
    var row = document.createElement("div");
    row.className = "finding-row";
    row.setAttribute("data-severity", f.severity || "");
    row.setAttribute("data-time", f.time || "");
    row.setAttribute("data-searchtext", [f.severity, f.rule_id, f.rule_name, f.evidence, f.tool].join(" "));
    var detailHTML = renderFindingDetail(f);
    var expandIcon = detailHTML ? '<span class="expand-icon">▸</span>' : '<span class="expand-icon-spacer"></span>';
    row.innerHTML =
      '<div class="row-summary">' +
        expandIcon +
        '<span class="finding-time">' + esc(f.time) + "</span>" +
        '<span class="sev-badge sev-' + esc(f.severity) + '">' + esc(f.severity) + "</span>" +
        '<span class="finding-rule">' + esc(f.rule_id) + "</span>" +
        '<span class="finding-evidence" title="' + escAttr(f.evidence) + '">' + esc(f.evidence) + "</span>" +
      '</div>' +
      detailHTML;
    if (detailHTML) {
      row.querySelector(".detail-panel").style.display = "none";
      row.querySelector(".row-summary").addEventListener("click", function () {
        toggleDetail(row);
        var icon = row.querySelector(".expand-icon");
        if (icon) icon.textContent = row.classList.contains("expanded") ? "▾" : "▸";
      });
      row.style.cursor = "pointer";
    }

    // Apply current filter
    var hasFilter = currentSearch || currentSeverity || currentDateFrom || currentDateTo;
    if (hasFilter) {
      var text = row.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (currentSearch && text.indexOf(currentSearch) === -1) show = false;
      if (currentSeverity && f.severity !== currentSeverity) show = false;
      if (show && (currentDateFrom || currentDateTo) && f.time) {
        var ts = new Date(f.time.replace(" ", "T")).getTime();
        var fromTs = currentDateFrom ? new Date(currentDateFrom).getTime() : 0;
        var toTs = currentDateTo ? new Date(currentDateTo).getTime() : Infinity;
        if (ts < fromTs || ts > toTs) show = false;
      }
      if (!show) row.classList.add("hidden");
    }

    findingList.appendChild(row);

    while (findingList.children.length > MAX_ROWS) {
      findingList.removeChild(findingList.firstChild);
    }

    if (findingAutoFollow) {
      scrollToBottom(findingList);
    }

    // Overview tracking
    ovTrackFinding(f);

    // Cross-reference: push finding into relevant access tabs with alert-level risk
    if (f.action_type && f.target) {
      var alertAction = {
        time: f.time,
        tool: f.tool || "",
        target: f.target,
        action_type: f.action_type,
        _alertRisk: f.severity === "CRITICAL" ? "critical" : f.severity === "HIGH" ? "high" : "medium",
        _alertReason: f.rule_id + ": " + f.rule_name,
        detail: { rule_id: f.rule_id, rule_name: f.rule_name, severity: f.severity, evidence: f.evidence, description: f.description, mitre: f.mitre }
      };
      trackFileAccessFromAlert(alertAction);
      trackToolUsageFromAlert(alertAction);
      trackProcessFromAlert(alertAction);
    }
  }

  function updateStats(s) {
    statSessions.textContent = s.sessions;
    statActions.textContent = s.actions;
    statFindings.textContent = s.findings;
  }

  // --- URL panel ---
  var urlTotal = 0;
  var urlSuspicious = 0;
  var urlCurSearch = "";
  var urlCurRisk = "";
  var urlCurSource = "";

  function updateURLSummary() {
    urlSummaryEl.textContent = urlTotal + " URLs accessed, " + urlSuspicious + " suspicious";
    statUrls.textContent = urlTotal;
  }

  urlFilterSearch.addEventListener("input", function () {
    urlCurSearch = this.value.toLowerCase();
    applyURLFilters();
  });

  urlFilterRisk.addEventListener("change", function () {
    urlCurRisk = this.value;
    applyURLFilters();
  });

  urlFilterSource.addEventListener("change", function () {
    urlCurSource = this.value;
    applyURLFilters();
  });

  urlFilterClear.addEventListener("click", function () {
    urlFilterSearch.value = "";
    urlFilterRisk.value = "";
    urlFilterSource.value = "";
    urlCurSearch = "";
    urlCurRisk = "";
    urlCurSource = "";
    applyURLFilters();
  });

  function applyURLFilters() {
    var rows = urlList.querySelectorAll("tr:not(.table-detail-row)");
    var total = 0;
    var visible = 0;
    rows.forEach(function (tr) {
      total++;
      var text = (tr.getAttribute("data-searchtext") || "").toLowerCase();
      var risk = tr.getAttribute("data-risk") || "";
      var source = tr.getAttribute("data-source") || "";
      var show = true;
      if (urlCurSearch && text.indexOf(urlCurSearch) === -1) show = false;
      if (urlCurRisk && risk !== urlCurRisk) show = false;
      if (urlCurSource && source !== urlCurSource) show = false;
      tr.style.display = show ? "" : "none";
      var next = tr.nextElementSibling;
      if (next && next.classList.contains("table-detail-row")) {
        next.style.display = show && tr.classList.contains("expanded") ? "table-row" : "none";
      }
      if (show) visible++;
    });
    var hasFilter = urlCurSearch || urlCurRisk || urlCurSource;
    urlFilterCount.textContent = hasFilter ? "Showing " + visible + "/" + total : "";
  }

  function addURL(u) {
    urlTotal++;
    if (u.suspicious) urlSuspicious++;
    updateURLSummary();

    var tr = document.createElement("tr");
    var riskClass = u.suspicious ? "risk-suspicious" : "risk-safe";
    var riskText = u.suspicious ? "⚠ Suspicious" : "✓ Safe";
    var riskVal = u.suspicious ? "suspicious" : "safe";
    var reasonText = (u.reasons && u.reasons.length > 0) ? u.reasons.join(", ") : "—";

    tr.setAttribute("data-searchtext", [u.url, u.domain, u.source, reasonText].join(" "));
    tr.setAttribute("data-risk", riskVal);
    tr.setAttribute("data-source", u.source || "");

    tr.innerHTML =
      '<td>' + esc(u.time) + '</td>' +
      '<td class="' + riskClass + '">' + riskText + '</td>' +
      '<td>' + esc(u.domain) + '</td>' +
      '<td class="url-cell"><a href="' + escAttr(u.url) + '" target="_blank" rel="noopener noreferrer">' + esc(u.url) + '</a></td>' +
      '<td>' + esc(u.source) + '</td>' +
      '<td class="url-reason">' + esc(reasonText) + '</td>';

    // Apply current filter to new row
    var hasFilter = urlCurSearch || urlCurRisk || urlCurSource;
    if (hasFilter) {
      var text = tr.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (urlCurSearch && text.indexOf(urlCurSearch) === -1) show = false;
      if (urlCurRisk && riskVal !== urlCurRisk) show = false;
      if (urlCurSource && (u.source || "") !== urlCurSource) show = false;
      if (!show) tr.style.display = "none";
    }

    urlList.appendChild(tr);
    // Build detail from URL data
    var urlDetail = {};
    if (u.url) urlDetail.url = u.url;
    if (u.domain) urlDetail.domain = u.domain;
    if (u.source) urlDetail.source = u.source;
    if (u.reasons && u.reasons.length) urlDetail.reasons = u.reasons.join(", ");
    attachTableDetail(tr, urlDetail, 6);
  }

  // --- Tool Usage tracking ---
  var toolTotal = 0;
  var toolDistinct = 0;
  var toolStats = {}; // { toolName: count }
  var knownToolFilter = {};
  var toolCurSearch = "";
  var toolCurName = "";
  var toolCurAction = "";
  var toolCurRisk = "";

  function classifyToolRisk(actionType) {
    switch (actionType) {
      case "terminal_exec": case "terminal_input": return "high";
      case "network_fetch": case "file_create": case "file_edit": return "medium";
      default: return "safe";
    }
  }

  function registerToolFilter(name) {
    if (!name || knownToolFilter[name]) return;
    knownToolFilter[name] = true;
    var opt = document.createElement("option");
    opt.value = name;
    opt.textContent = name;
    toolFilterName.appendChild(opt);
  }

  function trackToolUsage(a) {
    var tool = a.tool || "unknown";
    var ts = a.time || "";
    var target = a.target || "";
    var actionType = a.action_type || "";
    var risk = classifyToolRisk(actionType);

    registerToolFilter(tool);

    if (!toolStats[tool]) {
      toolStats[tool] = 0;
      toolDistinct++;
    }
    toolStats[tool]++;
    toolTotal++;

    statTools.textContent = toolTotal;
    toolSummaryEl.textContent = toolTotal + " tool calls across " + toolDistinct + " distinct tools";

    var targetDisplay = target.length > 80 ? target.substring(0, 80) + "…" : target;
    var riskClass = "risk-" + risk;
    var riskLabel = risk === "high" ? "🟠 HIGH" : risk === "medium" ? "⚠ MEDIUM" : "✓ Safe";

    var tr = document.createElement("tr");
    tr.setAttribute("data-searchtext", [tool, target, actionType].join(" "));
    tr.setAttribute("data-tool", tool);
    tr.setAttribute("data-action", actionType);
    tr.setAttribute("data-risk", risk);
    tr.innerHTML =
      '<td>' + esc(ts) + '</td>' +
      '<td class="' + riskClass + '">' + riskLabel + '</td>' +
      '<td><strong>' + esc(tool) + '</strong></td>' +
      '<td>' + esc(actionType) + '</td>' +
      '<td title="' + escAttr(target) + '">' + esc(targetDisplay) + '</td>' +
      '<td>' + toolStats[tool] + '</td>';

    // Apply current filter
    if (toolCurSearch || toolCurName || toolCurAction || toolCurRisk) {
      var text = tr.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (toolCurSearch && text.indexOf(toolCurSearch) === -1) show = false;
      if (toolCurName && tool !== toolCurName) show = false;
      if (toolCurAction && actionType !== toolCurAction) show = false;
      if (toolCurRisk && risk !== toolCurRisk) show = false;
      if (!show) tr.style.display = "none";
    }

    toolDetailList.appendChild(tr);
    attachTableDetail(tr, a.detail, 6);
    while (toolDetailList.children.length > 500) {
      toolDetailList.removeChild(toolDetailList.firstChild);
    }
  }

  toolFilterSearch.addEventListener("input", function () {
    toolCurSearch = this.value.toLowerCase();
    applyToolFilters();
  });

  toolFilterName.addEventListener("change", function () {
    toolCurName = this.value;
    applyToolFilters();
  });

  toolFilterAction.addEventListener("change", function () {
    toolCurAction = this.value;
    applyToolFilters();
  });

  toolFilterRisk.addEventListener("change", function () {
    toolCurRisk = this.value;
    applyToolFilters();
  });

  toolFilterClear.addEventListener("click", function () {
    toolFilterSearch.value = "";
    toolFilterName.value = "";
    toolFilterAction.value = "";
    toolFilterRisk.value = "";
    toolCurSearch = "";
    toolCurName = "";
    toolCurAction = "";
    toolCurRisk = "";
    applyToolFilters();
  });

  function applyToolFilters() {
    var rows = toolDetailList.querySelectorAll("tr:not(.table-detail-row)");
    var total = 0;
    var visible = 0;
    rows.forEach(function (tr) {
      total++;
      var text = (tr.getAttribute("data-searchtext") || "").toLowerCase();
      var tool = tr.getAttribute("data-tool") || "";
      var action = tr.getAttribute("data-action") || "";
      var risk = tr.getAttribute("data-risk") || "";
      var show = true;
      if (toolCurSearch && text.indexOf(toolCurSearch) === -1) show = false;
      if (toolCurName && tool !== toolCurName) show = false;
      if (toolCurAction && action !== toolCurAction) show = false;
      if (toolCurRisk && risk !== toolCurRisk) show = false;
      tr.style.display = show ? "" : "none";
      var next = tr.nextElementSibling;
      if (next && next.classList.contains("table-detail-row")) {
        next.style.display = show && tr.classList.contains("expanded") ? "table-row" : "none";
      }
      if (show) visible++;
    });
    var hasFilter = toolCurSearch || toolCurName || toolCurAction || toolCurRisk;
    toolFilterCount.textContent = hasFilter ? "Showing " + visible + "/" + total : "";
  }

  // --- Process Access tracking ---
  var procTotal = 0;
  var procSuspicious = 0;
  var procCurSearch = "";
  var procCurRisk = "";

  // Dangerous binary patterns
  var criticalBinaries = /^(nc|ncat|nmap|socat|msfconsole|msfvenom|hydra|john|hashcat|mimikatz)$/i;
  var highRiskPatterns = [
    { re: /curl\s+.*\|\s*(sh|bash|zsh)/, reason: "curl pipe to shell" },
    { re: /wget\s+.*\|\s*(sh|bash|zsh)/, reason: "wget pipe to shell" },
    { re: /python[23]?\s+-c/, reason: "inline Python execution" },
    { re: /node\s+-e/, reason: "inline Node.js execution" },
    { re: /eval\s/, reason: "eval usage" },
    { re: /base64\s+-d/, reason: "base64 decode" },
    { re: /chmod\s+777/, reason: "chmod 777" },
    { re: /rm\s+-rf\s+\//, reason: "recursive delete from root" },
    { re: /dd\s+if=/, reason: "raw disk access" },
    { re: /mkfs\./, reason: "filesystem format" },
  ];
  var safeBinaries = /^(go|git|make|cargo|npm|npx|yarn|pnpm|pip|uv|poetry|docker|kubectl|ls|cat|head|tail|wc|echo|cd|pwd|mkdir|cp|mv|grep|find|sort|uniq|awk|sed|jq|which|env|export|source|test)$/i;

  function extractBinary(cmd) {
    if (!cmd) return "";
    // Strip leading env vars, sudo, etc
    var c = cmd.replace(/^(sudo\s+|env\s+\S+=\S+\s+|time\s+|nice\s+)*/i, "").trim();
    // Get first token
    var parts = c.split(/\s+/);
    var bin = parts[0] || "";
    // Strip path prefix
    var slash = bin.lastIndexOf("/");
    if (slash >= 0) bin = bin.substring(slash + 1);
    return bin;
  }

  function classifyProcess(bin, fullCmd) {
    if (!bin) return { risk: "safe", reason: "—" };
    if (criticalBinaries.test(bin)) return { risk: "critical", reason: "dangerous binary: " + bin };
    for (var i = 0; i < highRiskPatterns.length; i++) {
      if (highRiskPatterns[i].re.test(fullCmd)) return { risk: "high", reason: highRiskPatterns[i].reason };
    }
    if (/curl|wget/.test(bin)) return { risk: "medium", reason: "network download tool" };
    if (safeBinaries.test(bin)) return { risk: "safe", reason: "—" };
    return { risk: "medium", reason: "unrecognized binary" };
  }

  function trackProcess(a) {
    if (a.action_type !== "terminal_exec" && a.action_type !== "terminal_input") return;
    var cmd = "";
    if (a.detail && a.detail.command) cmd = a.detail.command;
    else if (a.target) cmd = a.target;
    if (!cmd) return;

    // Handle chained commands (&&, |)
    var segments = cmd.split(/\s*(?:&&|\|)\s*/);
    segments.forEach(function (seg) {
      seg = seg.trim();
      if (!seg) return;
      var bin = extractBinary(seg);
      if (!bin) return;
      var cls = classifyProcess(bin, cmd);

      procTotal++;
      if (cls.risk !== "safe") procSuspicious++;
      statProcs.textContent = procTotal;
      procSummaryEl.textContent = procTotal + " processes observed, " + procSuspicious + " flagged";

      // Overview
      ovTrackProcRisk(bin, cmd, cls.risk);

      var tr = document.createElement("tr");
      var riskClass = "risk-" + cls.risk;
      var riskLabel = cls.risk === "critical" ? "🔴 CRITICAL" : cls.risk === "high" ? "🟠 HIGH" : cls.risk === "medium" ? "⚠ MEDIUM" : "✓ Safe";
      var cmdDisplay = cmd.length > 100 ? cmd.substring(0, 100) + "…" : cmd;

      tr.setAttribute("data-searchtext", [bin, cmd, cls.reason].join(" "));
      tr.setAttribute("data-risk", cls.risk);
      tr.innerHTML =
        '<td>' + esc(a.time) + '</td>' +
        '<td class="' + riskClass + '">' + riskLabel + '</td>' +
        '<td><strong>' + esc(bin) + '</strong></td>' +
        '<td title="' + escAttr(cmd) + '"><code>' + esc(cmdDisplay) + '</code></td>' +
        '<td class="url-reason">' + esc(cls.reason) + '</td>';

      // Apply filter
      if (procCurSearch || procCurRisk) {
        var text = tr.getAttribute("data-searchtext").toLowerCase();
        var show = true;
        if (procCurSearch && text.indexOf(procCurSearch) === -1) show = false;
        if (procCurRisk && cls.risk !== procCurRisk) show = false;
        if (!show) tr.style.display = "none";
      }

      procList.appendChild(tr);
      attachTableDetail(tr, a.detail, 5);
    });
  }

  procFilterSearch.addEventListener("input", function () {
    procCurSearch = this.value.toLowerCase();
    applyProcFilters();
  });
  procFilterRisk.addEventListener("change", function () {
    procCurRisk = this.value;
    applyProcFilters();
  });
  procFilterClear.addEventListener("click", function () {
    procFilterSearch.value = "";
    procFilterRisk.value = "";
    procCurSearch = "";
    procCurRisk = "";
    applyProcFilters();
  });
  function applyProcFilters() {
    var rows = procList.querySelectorAll("tr:not(.table-detail-row)");
    var total = 0, visible = 0;
    rows.forEach(function (tr) {
      total++;
      var text = (tr.getAttribute("data-searchtext") || "").toLowerCase();
      var risk = tr.getAttribute("data-risk") || "";
      var show = true;
      if (procCurSearch && text.indexOf(procCurSearch) === -1) show = false;
      if (procCurRisk && risk !== procCurRisk) show = false;
      tr.style.display = show ? "" : "none";
      var next = tr.nextElementSibling;
      if (next && next.classList.contains("table-detail-row")) {
        next.style.display = show && tr.classList.contains("expanded") ? "table-row" : "none";
      }
      if (show) visible++;
    });
    procFilterCount.textContent = (procCurSearch || procCurRisk) ? "Showing " + visible + "/" + total : "";
  }

  // --- File Access tracking ---
  var fileTotal = 0;
  var fileSuspicious = 0;
  var fileCurSearch = "";
  var fileCurOp = "";
  var fileCurRisk = "";

  var sensitivePathRe = /(?:\.ssh|credentials|\.kube\/config|\.docker\/config|id_rsa|id_ed25519|\.pem$|\.netrc|\.pgpass|\.gnupg|authorized_keys)/i;
  var secretFileRe = /(?:\.env|\.aws|known_hosts)/i;
  var backdoorPathRe = /(?:\.bashrc|\.bash_profile|\.profile|\.zshrc|\.zprofile|crontab|\.gitconfig)/i;
  var historyPathRe = /(?:\.bash_history|\.zsh_history|\.node_repl_history|\.python_history)/i;
  var systemCredsRe = /(?:\/etc\/passwd|\/etc\/shadow|\/etc\/sudoers|\/etc\/group)/i;

  function classifyFileAccess(path, op) {
    if (!path) return { risk: "safe", reason: "\u2014" };
    var isWrite = (op === "create" || op === "edit");

    // Critical: writing to auth/backdoor targets
    if (isWrite && sensitivePathRe.test(path)) return { risk: "critical", reason: "write to auth/key file" };
    if (isWrite && backdoorPathRe.test(path)) return { risk: "critical", reason: "write to shell config (backdoor risk)" };
    if (isWrite && /\/tmp\//.test(path)) return { risk: "critical", reason: "write to /tmp (staging area)" };

    // High: reading secrets/keys, writing .env
    if (sensitivePathRe.test(path)) return { risk: "high", reason: "read auth/key file" };
    if (isWrite && secretFileRe.test(path)) return { risk: "high", reason: "write to secret/config" };
    if (systemCredsRe.test(path)) return { risk: "high", reason: "system credential file" };

    // Medium: reading secrets, history, system paths
    if (secretFileRe.test(path)) return { risk: "medium", reason: "read secret/config" };
    if (historyPathRe.test(path)) return { risk: "medium", reason: "shell history (contains commands)" };
    if (backdoorPathRe.test(path)) return { risk: "medium", reason: "read shell config" };
    if (/^\/(etc|proc|sys)\//.test(path)) return { risk: "medium", reason: "system path" };

    return { risk: "safe", reason: "—" };
  }

  function mapFileOp(actionType) {
    switch (actionType) {
      case "file_read": return "read";
      case "file_create": return "create";
      case "file_edit": return "edit";
      case "dir_list": return "list";
      case "file_search": case "text_search": case "semantic_search": return "search";
      default: return "";
    }
  }

  function trackFileAccess(a) {
    var op = mapFileOp(a.action_type);
    if (!op) return;

    var path = a.target || "";
    if (!path) return;

    fileTotal++;
    var cls = classifyFileAccess(path, op);
    if (cls.risk !== "safe") fileSuspicious++;
    statFiles.textContent = fileTotal;
    fileSummaryEl.textContent = fileTotal + " file operations, " + fileSuspicious + " flagged";

    // Overview
    ovTrackFileRisk(path, op, cls.risk);

    var tr = document.createElement("tr");
    var opClass = "op-" + op;
    var riskClass = "risk-" + cls.risk;
    var riskLabel = cls.risk === "critical" ? "\ud83d\udd34 CRITICAL" : cls.risk === "high" ? "\ud83d\udfe0 HIGH" : cls.risk === "medium" ? "\u26a0 MEDIUM" : "\u2713 Safe";
    var pathDisplay = path.length > 80 ? "…" + path.substring(path.length - 77) : path;

    tr.setAttribute("data-searchtext", [path, op, a.tool, cls.reason].join(" "));
    tr.setAttribute("data-op", op);
    tr.setAttribute("data-risk", cls.risk);
    tr.innerHTML =
      '<td>' + esc(a.time) + '</td>' +
      '<td class="' + opClass + '">' + op.toUpperCase() + '</td>' +
      '<td class="' + riskClass + '">' + riskLabel + '</td>' +
      '<td class="path-cell" title="' + escAttr(path) + '">' + esc(pathDisplay) + '</td>' +
      '<td>' + esc(a.tool) + '</td>' +
      '<td class="url-reason">' + esc(cls.reason) + '</td>';

    // Apply filter
    if (fileCurSearch || fileCurOp || fileCurRisk) {
      var text = tr.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (fileCurSearch && text.indexOf(fileCurSearch) === -1) show = false;
      if (fileCurOp && op !== fileCurOp) show = false;
      if (fileCurRisk && cls.risk !== fileCurRisk) show = false;
      if (!show) tr.style.display = "none";
    }

    fileList.appendChild(tr);
    attachTableDetail(tr, a.detail, 6);
    while (fileList.children.length > 1000) {
      fileList.removeChild(fileList.firstChild);
    }
  }

  fileFilterSearch.addEventListener("input", function () {
    fileCurSearch = this.value.toLowerCase();
    applyFileFilters();
  });
  fileFilterOp.addEventListener("change", function () {
    fileCurOp = this.value;
    applyFileFilters();
  });
  fileFilterRisk.addEventListener("change", function () {
    fileCurRisk = this.value;
    applyFileFilters();
  });
  fileFilterClear.addEventListener("click", function () {
    fileFilterSearch.value = "";
    fileFilterOp.value = "";
    fileFilterRisk.value = "";
    fileCurSearch = "";
    fileCurOp = "";
    fileCurRisk = "";
    applyFileFilters();
  });
  function applyFileFilters() {
    var rows = fileList.querySelectorAll("tr:not(.table-detail-row)");
    var total = 0, visible = 0;
    rows.forEach(function (tr) {
      total++;
      var text = (tr.getAttribute("data-searchtext") || "").toLowerCase();
      var op = tr.getAttribute("data-op") || "";
      var risk = tr.getAttribute("data-risk") || "";
      var show = true;
      if (fileCurSearch && text.indexOf(fileCurSearch) === -1) show = false;
      if (fileCurOp && op !== fileCurOp) show = false;
      if (fileCurRisk && risk !== fileCurRisk) show = false;
      tr.style.display = show ? "" : "none";
      var next = tr.nextElementSibling;
      if (next && next.classList.contains("table-detail-row")) {
        next.style.display = show && tr.classList.contains("expanded") ? "table-row" : "none";
      }
      if (show) visible++;
    });
    fileFilterCount.textContent = (fileCurSearch || fileCurOp || fileCurRisk) ? "Showing " + visible + "/" + total : "";
  }

  // --- Alert cross-reference: inject finding into access tabs with alert-level risk ---
  function trackFileAccessFromAlert(a) {
    var op = mapFileOp(a.action_type);
    if (!op) return;
    var path = a.target || "";
    if (!path) return;

    var risk = a._alertRisk;
    var reason = a._alertReason;

    fileTotal++;
    fileSuspicious++;
    statFiles.textContent = fileTotal;
    fileSummaryEl.textContent = fileTotal + " file operations, " + fileSuspicious + " flagged";

    var tr = document.createElement("tr");
    var opClass = "op-" + op;
    var riskClass = "risk-" + risk;
    var riskLabel = risk === "critical" ? "🔴 CRITICAL" : risk === "high" ? "🟠 HIGH" : "⚠ MEDIUM";
    var pathDisplay = path.length > 80 ? "…" + path.substring(path.length - 77) : path;

    tr.setAttribute("data-searchtext", [path, op, a.tool, reason].join(" "));
    tr.setAttribute("data-op", op);
    tr.setAttribute("data-risk", risk);
    tr.innerHTML =
      '<td>' + esc(a.time) + '</td>' +
      '<td class="' + opClass + '">' + op.toUpperCase() + '</td>' +
      '<td class="' + riskClass + '">' + riskLabel + '</td>' +
      '<td class="path-cell" title="' + escAttr(path) + '">' + esc(pathDisplay) + '</td>' +
      '<td>' + esc(a.tool) + '</td>' +
      '<td class="url-reason">🚨 ' + esc(reason) + '</td>';

    if (a.detail) attachTableDetail(tr, a.detail, 6);

    if (fileCurSearch || fileCurOp || fileCurRisk) {
      var text = tr.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (fileCurSearch && text.indexOf(fileCurSearch) === -1) show = false;
      if (fileCurOp && op !== fileCurOp) show = false;
      if (fileCurRisk && risk !== fileCurRisk) show = false;
      if (!show) tr.style.display = "none";
    }
    fileList.appendChild(tr);
  }

  function trackToolUsageFromAlert(a) {
    var tool = a.tool || "unknown";
    var risk = a._alertRisk;
    var reason = a._alertReason;
    var actionType = a.action_type || "";
    var target = a.target || "";

    registerToolFilter(tool);
    if (!toolStats[tool]) { toolStats[tool] = 0; toolDistinct++; }
    toolStats[tool]++;
    toolTotal++;
    statTools.textContent = toolTotal;
    toolSummaryEl.textContent = toolTotal + " tool calls across " + toolDistinct + " distinct tools";

    var targetDisplay = target.length > 80 ? target.substring(0, 80) + "…" : target;
    var riskClass = "risk-" + risk;
    var riskLabel = risk === "critical" ? "🔴 CRITICAL" : risk === "high" ? "🟠 HIGH" : "⚠ MEDIUM";

    var tr = document.createElement("tr");
    tr.setAttribute("data-searchtext", [tool, target, actionType, reason].join(" "));
    tr.setAttribute("data-tool", tool);
    tr.setAttribute("data-action", actionType);
    tr.setAttribute("data-risk", risk);
    tr.innerHTML =
      '<td>' + esc(a.time) + '</td>' +
      '<td class="' + riskClass + '">' + riskLabel + '</td>' +
      '<td><strong>' + esc(tool) + '</strong></td>' +
      '<td>' + esc(actionType) + '</td>' +
      '<td title="' + escAttr(target) + '">🚨 ' + esc(targetDisplay) + '</td>' +
      '<td>' + toolStats[tool] + '</td>';

    if (a.detail) attachTableDetail(tr, a.detail, 6);

    if (toolCurSearch || toolCurName || toolCurAction || toolCurRisk) {
      var text = tr.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (toolCurSearch && text.indexOf(toolCurSearch) === -1) show = false;
      if (toolCurName && tool !== toolCurName) show = false;
      if (toolCurAction && actionType !== toolCurAction) show = false;
      if (toolCurRisk && risk !== toolCurRisk) show = false;
      if (!show) tr.style.display = "none";
    }
    toolDetailList.appendChild(tr);
  }

  function trackProcessFromAlert(a) {
    if (a.action_type !== "terminal_exec" && a.action_type !== "terminal_input") return;
    var cmd = a.target || "";
    if (!cmd) return;
    var bin = extractBinary(cmd);
    if (!bin) return;

    var risk = a._alertRisk;
    var reason = a._alertReason;

    procTotal++;
    procSuspicious++;
    statProcs.textContent = procTotal;
    procSummaryEl.textContent = procTotal + " processes observed, " + procSuspicious + " flagged";

    var tr = document.createElement("tr");
    var riskClass = "risk-" + risk;
    var riskLabel = risk === "critical" ? "🔴 CRITICAL" : risk === "high" ? "🟠 HIGH" : "⚠ MEDIUM";
    var cmdDisplay = cmd.length > 100 ? cmd.substring(0, 100) + "…" : cmd;

    tr.setAttribute("data-searchtext", [bin, cmd, reason].join(" "));
    tr.setAttribute("data-risk", risk);
    tr.innerHTML =
      '<td>' + esc(a.time) + '</td>' +
      '<td class="' + riskClass + '">' + riskLabel + '</td>' +
      '<td><strong>' + esc(bin) + '</strong></td>' +
      '<td title="' + escAttr(cmd) + '"><code>' + esc(cmdDisplay) + '</code></td>' +
      '<td class="url-reason">🚨 ' + esc(reason) + '</td>';

    if (a.detail) attachTableDetail(tr, a.detail, 5);

    if (procCurSearch || procCurRisk) {
      var text = tr.getAttribute("data-searchtext").toLowerCase();
      var show = true;
      if (procCurSearch && text.indexOf(procCurSearch) === -1) show = false;
      if (procCurRisk && risk !== procCurRisk) show = false;
      if (!show) tr.style.display = "none";
    }
    procList.appendChild(tr);
  }

  // --- Table row detail expand ---
  function attachTableDetail(tr, detail, colSpan) {
    if (!detail || Object.keys(detail).length === 0) return;
    var detailTr = document.createElement("tr");
    detailTr.className = "table-detail-row";
    detailTr.style.display = "none";
    var td = document.createElement("td");
    td.colSpan = colSpan;
    td.innerHTML = renderDetail(detail);
    // Show the inner panel directly
    var panel = td.querySelector(".detail-panel");
    if (panel) panel.style.display = "block";
    detailTr.appendChild(td);

    tr.style.cursor = "pointer";
    tr.classList.add("expandable");
    tr.addEventListener("click", function () {
      var open = detailTr.style.display !== "none";
      detailTr.style.display = open ? "none" : "table-row";
      tr.classList.toggle("expanded", !open);
    });

    // Insert detail row right after tr
    if (tr.parentNode) {
      tr.parentNode.insertBefore(detailTr, tr.nextSibling);
    } else {
      // Defer until tr is in DOM
      var obs = new MutationObserver(function () {
        if (tr.parentNode) {
          tr.parentNode.insertBefore(detailTr, tr.nextSibling);
          obs.disconnect();
        }
      });
      obs.observe(document.body, { childList: true, subtree: true });
    }
  }

  // --- Escape helpers ---
  function esc(s) {
    if (!s) return "";
    var d = document.createElement("div");
    d.appendChild(document.createTextNode(s));
    return d.innerHTML;
  }

  function escAttr(s) {
    return esc(s).replace(/"/g, "&quot;");
  }

  // --- Overview tab ---
  var ovActions = document.getElementById("ov-actions");
  var ovFindings = document.getElementById("ov-findings");
  var ovCritical = document.getElementById("ov-critical");
  var ovHigh = document.getElementById("ov-high");
  var ovMedium = document.getElementById("ov-medium");
  var ovLow = document.getElementById("ov-low");
  var ovTopFiles = document.getElementById("ov-top-files");
  var ovTopProcs = document.getElementById("ov-top-procs");
  var ovTopTools = document.getElementById("ov-top-tools");
  var ovFindingsList = document.getElementById("ov-findings-list");

  var ovActionCount = 0;
  var ovFindingCount = 0;
  var ovSevCounts = { CRITICAL: 0, HIGH: 0, MEDIUM: 0, LOW: 0 };
  var ovFileRisks = []; // {risk, path, op}
  var ovProcRisks = []; // {risk, binary, command}
  var ovToolCounts = {}; // tool -> count

  var riskOrder = { critical: 0, high: 1, medium: 2, suspicious: 3, safe: 4 };

  function ovTrackAction(a) {
    ovActionCount++;
    ovActions.textContent = ovActionCount;

    // Track tool counts
    var tool = a.tool || "unknown";
    ovToolCounts[tool] = (ovToolCounts[tool] || 0) + 1;
    ovRenderTopTools();
  }

  function ovTrackFinding(f) {
    ovFindingCount++;
    ovFindings.textContent = ovFindingCount;

    var sev = f.severity || "LOW";
    if (ovSevCounts[sev] !== undefined) ovSevCounts[sev]++;
    ovCritical.textContent = ovSevCounts.CRITICAL;
    ovHigh.textContent = ovSevCounts.HIGH;
    ovMedium.textContent = ovSevCounts.MEDIUM;
    ovLow.textContent = ovSevCounts.LOW;

    ovAppendFinding(f);
  }

  function ovTrackFileRisk(path, op, risk) {
    ovFileRisks.push({ risk: risk, path: path, op: op });
    ovRenderTopFiles();
  }

  function ovTrackProcRisk(binary, command, risk) {
    ovProcRisks.push({ risk: risk, binary: binary, command: command });
    ovRenderTopProcs();
  }

  function ovRenderTopFiles() {
    var sorted = ovFileRisks.slice().sort(function (a, b) {
      return (riskOrder[a.risk] || 9) - (riskOrder[b.risk] || 9);
    });
    var top5 = sorted.slice(0, 5);
    ovTopFiles.innerHTML = "";
    if (top5.length === 0) {
      ovTopFiles.innerHTML = '<tr><td colspan="3" class="ov-empty">No file access yet</td></tr>';
      return;
    }
    top5.forEach(function (f) {
      var tr = document.createElement("tr");
      tr.innerHTML = '<td><span class="risk-' + esc(f.risk) + '">' + esc(f.risk) + '</span></td>'
        + '<td title="' + escAttr(f.path) + '">' + esc(f.path) + '</td>'
        + '<td>' + esc(f.op) + '</td>';
      ovTopFiles.appendChild(tr);
    });
  }

  function ovRenderTopProcs() {
    var sorted = ovProcRisks.slice().sort(function (a, b) {
      return (riskOrder[a.risk] || 9) - (riskOrder[b.risk] || 9);
    });
    var top5 = sorted.slice(0, 5);
    ovTopProcs.innerHTML = "";
    if (top5.length === 0) {
      ovTopProcs.innerHTML = '<tr><td colspan="3" class="ov-empty">No processes yet</td></tr>';
      return;
    }
    top5.forEach(function (p) {
      var tr = document.createElement("tr");
      var cmd = p.command || "";
      if (cmd.length > 60) cmd = cmd.substring(0, 60) + "...";
      tr.innerHTML = '<td><span class="risk-' + esc(p.risk) + '">' + esc(p.risk) + '</span></td>'
        + '<td>' + esc(p.binary) + '</td>'
        + '<td title="' + escAttr(p.command) + '">' + esc(cmd) + '</td>';
      ovTopProcs.appendChild(tr);
    });
  }

  function ovRenderTopTools() {
    var entries = Object.keys(ovToolCounts).map(function (k) {
      return { tool: k, count: ovToolCounts[k] };
    });
    entries.sort(function (a, b) { return b.count - a.count; });
    var top5 = entries.slice(0, 5);
    ovTopTools.innerHTML = "";
    if (top5.length === 0) {
      ovTopTools.innerHTML = '<tr><td colspan="2" class="ov-empty">No tools yet</td></tr>';
      return;
    }
    top5.forEach(function (t) {
      var tr = document.createElement("tr");
      tr.innerHTML = '<td>' + esc(t.tool) + '</td><td>' + t.count + '</td>';
      ovTopTools.appendChild(tr);
    });
  }

  function ovRenderFindings() {
    ovFindingsList.innerHTML = '<tr><td colspan="5" class="ov-empty">No findings</td></tr>';
  }

  function ovAppendFinding(f) {
    // Remove empty placeholder if present
    var empty = ovFindingsList.querySelector(".ov-empty");
    if (empty) empty.closest("tr").remove();

    var tr = document.createElement("tr");
    var ev = f.evidence || "";
    if (ev.length > 60) ev = ev.substring(0, 60) + "...";
    tr.innerHTML = '<td>' + esc(f.time) + '</td>'
      + '<td class="sev-' + esc(f.severity) + '">' + esc(f.severity) + '</td>'
      + '<td>' + esc(f.rule_id) + ' ' + esc(f.rule_name) + '</td>'
      + '<td>' + esc(f.tool) + '</td>'
      + '<td title="' + escAttr(f.evidence) + '">' + esc(ev) + '</td>';

    // Insert sorted by severity: CRITICAL < HIGH < MEDIUM < LOW
    var sevOrder = { CRITICAL: 0, HIGH: 1, MEDIUM: 2, LOW: 3 };
    var fOrd = sevOrder[f.severity] !== undefined ? sevOrder[f.severity] : 9;
    var inserted = false;
    var rows = ovFindingsList.querySelectorAll("tr:not(.table-detail-row)");
    for (var i = 0; i < rows.length; i++) {
      var rowSev = rows[i].querySelector("td:nth-child(2)");
      if (rowSev) {
        var rowOrd = sevOrder[rowSev.textContent.trim()];
        if (rowOrd === undefined) rowOrd = 9;
        if (fOrd < rowOrd) {
          ovFindingsList.insertBefore(tr, rows[i]);
          inserted = true;
          break;
        }
      }
    }
    if (!inserted) ovFindingsList.appendChild(tr);

    var detail = {
      rule_id: f.rule_id, rule_name: f.rule_name, severity: f.severity,
      evidence: f.evidence, description: f.description, mitre: f.mitre,
      tool: f.tool, target: f.target, action_type: f.action_type
    };
    attachTableDetail(tr, detail, 5);
  }

  // Initialize empty overview
  ovRenderTopFiles();
  ovRenderTopProcs();
  ovRenderTopTools();
  ovRenderFindings();

  // --- Load history for late-joining clients ---
  function loadHistory() {
    fetch("/api/history")
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (data.actions) {
          data.actions.forEach(addAction);
        }
        if (data.findings) {
          data.findings.forEach(addFinding);
        }
        if (data.urls) {
          data.urls.forEach(addURL);
        }
      })
      .catch(function () {});

    fetch("/api/stats")
      .then(function (r) { return r.json(); })
      .then(updateStats)
      .catch(function () {});
  }

  // --- SSE connection ---
  function connectSSE() {
    var es = new EventSource("/api/events");

    es.onopen = function () {
      modeBadge.textContent = "● Live";
      modeBadge.className = "badge live";
    };

    es.onerror = function () {
      modeBadge.textContent = "● Reconnecting...";
      modeBadge.className = "badge";
    };

    es.onmessage = function (e) {
      var msg;
      try {
        msg = JSON.parse(e.data);
      } catch (_) {
        return;
      }

      switch (msg.type) {
        case "action":
          addAction(msg.payload);
          statActions.textContent = parseInt(statActions.textContent || "0", 10) + 1;
          break;
        case "finding":
          addFinding(msg.payload);
          statFindings.textContent = parseInt(statFindings.textContent || "0", 10) + 1;
          break;
        case "url":
          addURL(msg.payload);
          break;
        case "stats":
          updateStats(msg.payload);
          break;
      }
    };
  }

  // --- Init ---
  loadHistory();
  connectSSE();
})();
