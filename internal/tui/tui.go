package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aigaze-sec/aigaze/internal/engine"
	"github.com/aigaze-sec/aigaze/internal/parser"
	"github.com/aigaze-sec/aigaze/internal/watcher"
)

const maxRows = 500

// panel focus
const (
	panelActions  = 0
	panelFindings = 1
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	statsStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("244"))

	headerActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	rowHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("232")).
				Background(lipgloss.Color("39"))

	scrollInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

	detailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("109"))

	rowMatchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true)

	sevCRIT = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("🔴 CRIT")
	sevHIGH = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")).Render("🟠 HIGH")
	sevMED  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("⚠  MED")
	sevLOW  = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("ℹ  LOW")
)

func sevLabel(sev string) string {
	switch sev {
	case "CRITICAL":
		return sevCRIT
	case "HIGH":
		return sevHIGH
	case "MEDIUM":
		return sevMED
	case "LOW":
		return sevLOW
	default:
		return sev
	}
}

// Messages
type actionMsg struct {
	sessionID string
	action    parser.Action
}

type findingMsg struct {
	sessionID string
	finding   engine.Finding
}

type tickMsg time.Time
type replayDoneMsg struct{}

// actionRow stores a rendered action for display.
type actionRow struct {
	time      string
	sessionID string
	tool      string
	target    string
	detail    map[string]interface{} // full arguments for expand
	expanded  bool
}

type findingRow struct {
	time        string
	severity    string
	ruleID      string
	evidence    string
	description string
	mitre       string
	tool        string
	expanded    bool
}

// Model is the bubbletea model for the watch TUI.
type Model struct {
	engine     watcher.EventSource
	sessions   int
	actions    []actionRow
	findings   []findingRow
	width      int
	height     int
	quitting   bool
	replayMode bool

	// scroll state
	focusPanel    int  // panelActions or panelFindings
	actionOffset  int  // scroll offset for actions panel
	findingOffset int  // scroll offset for findings panel
	autoFollow    bool // auto-scroll to bottom on new data

	// cursor for expand
	actionCursor  int // index in actions slice
	findingCursor int // index in findings slice

	// search/filter
	searchMode            bool
	searchQuery           string
	filterIndicesActions  []int // indices into m.actions matching search
	filterIndicesFindings []int // indices into m.findings matching search
}

// NewModel creates a new TUI model from any EventSource.
func NewModel(src watcher.EventSource, sessionCount int, replayMode bool) Model {
	return Model{
		engine:     src,
		sessions:   sessionCount,
		replayMode: replayMode,
		autoFollow: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenActions(m.engine),
		listenFindings(m.engine),
		tickCmd(),
	)
}

// viewportHeight returns how many data rows fit in a panel.
func (m Model) viewportHeight() int {
	h := m.height
	if h == 0 {
		h = 40
	}
	// title(1) + stats(1) + blank(1) + header(1) + scrollbar-hint(1) + bottom-padding(1) = 6 lines overhead
	vp := h - 6
	if vp < 3 {
		vp = 3
	}
	return vp
}

// clampOffset ensures offset is within valid range.
func clampOffset(offset, totalRows, viewportH int) int {
	maxOff := totalRows - viewportH
	if maxOff < 0 {
		maxOff = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	vp := m.viewportHeight()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Search mode: capture typed characters
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchQuery = ""
				m.filterIndicesActions = nil
				m.filterIndicesFindings = nil
				return m, nil
			case "enter":
				m.searchMode = false
				// keep filter active, jump to first match
				if m.focusPanel == panelActions && len(m.filterIndicesActions) > 0 {
					m.actionCursor = m.filterIndicesActions[0]
					m.actionOffset = m.actionCursor
					m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
				} else if m.focusPanel == panelFindings && len(m.filterIndicesFindings) > 0 {
					m.findingCursor = m.filterIndicesFindings[0]
					m.findingOffset = m.findingCursor
					m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
				}
				return m, nil
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					updateSearchFilter(&m)
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
					updateSearchFilter(&m)
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.engine.Stop()
			return m, tea.Quit
		case "c":
			m.actions = nil
			m.findings = nil
			m.actionOffset = 0
			m.findingOffset = 0
			return m, nil
		case "tab":
			m.focusPanel = (m.focusPanel + 1) % 2
			return m, nil

		// Scroll keys
		case "j", "down":
			m.autoFollow = false
			if m.focusPanel == panelActions {
				if m.actionCursor < len(m.actions)-1 {
					m.actionCursor++
				}
				// scroll to keep cursor visible
				if m.actionCursor >= m.actionOffset+vp {
					m.actionOffset = m.actionCursor - vp + 1
				}
				m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
			} else {
				if m.findingCursor < len(m.findings)-1 {
					m.findingCursor++
				}
				if m.findingCursor >= m.findingOffset+vp {
					m.findingOffset = m.findingCursor - vp + 1
				}
				m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
			}
			return m, nil
		case "k", "up":
			m.autoFollow = false
			if m.focusPanel == panelActions {
				if m.actionCursor > 0 {
					m.actionCursor--
				}
				if m.actionCursor < m.actionOffset {
					m.actionOffset = m.actionCursor
				}
				m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
			} else {
				if m.findingCursor > 0 {
					m.findingCursor--
				}
				if m.findingCursor < m.findingOffset {
					m.findingOffset = m.findingCursor
				}
				m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
			}
			return m, nil
		case "enter":
			// Toggle expand on cursor row
			if m.focusPanel == panelActions && len(m.actions) > 0 {
				if m.actionCursor >= 0 && m.actionCursor < len(m.actions) {
					m.actions[m.actionCursor].expanded = !m.actions[m.actionCursor].expanded
				}
			} else if m.focusPanel == panelFindings && len(m.findings) > 0 {
				if m.findingCursor >= 0 && m.findingCursor < len(m.findings) {
					m.findings[m.findingCursor].expanded = !m.findings[m.findingCursor].expanded
				}
			}
			return m, nil
		case "pgdown", "ctrl+d":
			m.autoFollow = false
			step := vp / 2
			if step < 1 {
				step = 1
			}
			if m.focusPanel == panelActions {
				m.actionOffset += step
				m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
			} else {
				m.findingOffset += step
				m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
			}
			return m, nil
		case "pgup", "ctrl+u":
			m.autoFollow = false
			step := vp / 2
			if step < 1 {
				step = 1
			}
			if m.focusPanel == panelActions {
				m.actionOffset -= step
				m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
			} else {
				m.findingOffset -= step
				m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
			}
			return m, nil
		case "home", "g":
			m.autoFollow = false
			if m.focusPanel == panelActions {
				m.actionOffset = 0
			} else {
				m.findingOffset = 0
			}
			return m, nil
		case "end", "G":
			m.autoFollow = true
			if m.focusPanel == panelActions {
				m.actionOffset = clampOffset(len(m.actions), len(m.actions), vp)
				m.actionCursor = len(m.actions) - 1
			} else {
				m.findingOffset = clampOffset(len(m.findings), len(m.findings), vp)
				m.findingCursor = len(m.findings) - 1
			}
			return m, nil
		case "f":
			// Toggle auto-follow
			m.autoFollow = !m.autoFollow
			if m.autoFollow {
				m.actionOffset = clampOffset(len(m.actions), len(m.actions), vp)
				m.findingOffset = clampOffset(len(m.findings), len(m.findings), vp)
			}
			return m, nil
		case "/":
			m.searchMode = true
			m.searchQuery = ""
			m.autoFollow = false
			return m, nil
		case "esc":
			// Clear search filter
			m.searchQuery = ""
			m.filterIndicesActions = nil
			m.filterIndicesFindings = nil
			return m, nil
		case "n":
			// Jump to next match
			jumpToMatch(&m, 1, vp)
			return m, nil
		case "N":
			// Jump to previous match
			jumpToMatch(&m, -1, vp)
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case actionMsg:
		now := time.Now().Format("01-02 15:04:05")
		target := msg.action.Target
		if len(target) > 80 {
			target = target[:80]
		}
		m.actions = append(m.actions, actionRow{
			time:      now,
			sessionID: msg.sessionID,
			tool:      msg.action.Tool,
			target:    target,
			detail:    msg.action.Arguments,
		})
		if len(m.actions) > maxRows {
			dropped := len(m.actions) - maxRows
			m.actions = m.actions[dropped:]
			m.actionOffset -= dropped
			if m.actionOffset < 0 {
				m.actionOffset = 0
			}
		}
		if m.autoFollow {
			m.actionOffset = clampOffset(len(m.actions), len(m.actions), vp)
			m.actionCursor = len(m.actions) - 1
		}
		return m, listenActions(m.engine)

	case replayDoneMsg:
		return m, nil

	case findingMsg:
		now := time.Now().Format("01-02 15:04:05")
		evidence := msg.finding.Evidence
		if len(evidence) > 60 {
			evidence = evidence[:60]
		}
		m.findings = append(m.findings, findingRow{
			time:        now,
			severity:    msg.finding.Severity,
			ruleID:      msg.finding.RuleID,
			evidence:    evidence,
			description: msg.finding.Description,
			mitre:       msg.finding.MITRETechnique + " " + msg.finding.MITREName,
			tool:        msg.finding.Tool,
		})
		if len(m.findings) > maxRows {
			dropped := len(m.findings) - maxRows
			m.findings = m.findings[dropped:]
			m.findingOffset -= dropped
			if m.findingOffset < 0 {
				m.findingOffset = 0
			}
		}
		if m.autoFollow {
			m.findingOffset = clampOffset(len(m.findings), len(m.findings), vp)
			m.findingCursor = len(m.findings) - 1
		}
		return m, listenFindings(m.engine)

	case tickMsg:
		return m, tickCmd()
	}

	return m, nil
}

// updateSearchFilter recalculates the filter indices based on searchQuery.
func updateSearchFilter(m *Model) {
	q := strings.ToLower(m.searchQuery)
	m.filterIndicesActions = nil
	m.filterIndicesFindings = nil
	if q == "" {
		return
	}
	for i, row := range m.actions {
		text := strings.ToLower(row.tool + " " + row.target + " " + row.sessionID)
		if strings.Contains(text, q) {
			m.filterIndicesActions = append(m.filterIndicesActions, i)
		}
	}
	for i, row := range m.findings {
		text := strings.ToLower(row.ruleID + " " + row.severity + " " + row.evidence + " " + row.tool)
		if strings.Contains(text, q) {
			m.filterIndicesFindings = append(m.filterIndicesFindings, i)
		}
	}
}

// jumpToMatch moves cursor to next/prev search match. dir=1 forward, dir=-1 backward.
func jumpToMatch(m *Model, dir int, vp int) {
	if m.focusPanel == panelActions {
		indices := m.filterIndicesActions
		if len(indices) == 0 {
			return
		}
		if dir > 0 {
			for _, idx := range indices {
				if idx > m.actionCursor {
					m.actionCursor = idx
					m.actionOffset = idx
					m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
					return
				}
			}
			// Wrap
			m.actionCursor = indices[0]
			m.actionOffset = indices[0]
			m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
		} else {
			for i := len(indices) - 1; i >= 0; i-- {
				if indices[i] < m.actionCursor {
					m.actionCursor = indices[i]
					m.actionOffset = indices[i]
					m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
					return
				}
			}
			// Wrap
			m.actionCursor = indices[len(indices)-1]
			m.actionOffset = indices[len(indices)-1]
			m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
		}
	} else {
		indices := m.filterIndicesFindings
		if len(indices) == 0 {
			return
		}
		if dir > 0 {
			for _, idx := range indices {
				if idx > m.findingCursor {
					m.findingCursor = idx
					m.findingOffset = idx
					m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
					return
				}
			}
			m.findingCursor = indices[0]
			m.findingOffset = indices[0]
			m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
		} else {
			for i := len(indices) - 1; i >= 0; i-- {
				if indices[i] < m.findingCursor {
					m.findingCursor = indices[i]
					m.findingOffset = indices[i]
					m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
					return
				}
			}
			m.findingCursor = indices[len(indices)-1]
			m.findingOffset = indices[len(indices)-1]
			m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
		}
	}
}

// isMatchedRow returns true if the row index is in the filter match set.
func isMatchedRow(indices []int, idx int) bool {
	for _, i := range indices {
		if i == idx {
			return true
		}
	}
	return false
}

func (m Model) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	w := m.width
	if w == 0 {
		w = 120
	}
	vp := m.viewportHeight()

	// Title
	var titleText string
	if m.replayMode {
		titleText = "  AIGaze Watch — Offline Replay"
	} else {
		titleText = "  AIGaze Watch — Real-time AI Agent Action Monitor"
	}
	title := titleStyle.Render(titleText)

	// Stats bar
	followTag := ""
	if m.autoFollow {
		followTag = "  [auto-follow ON]"
	}
	searchTag := ""
	if m.searchMode {
		searchTag = fmt.Sprintf("  🔍 /%s▌", m.searchQuery)
	} else if m.searchQuery != "" {
		matchCount := len(m.filterIndicesActions) + len(m.filterIndicesFindings)
		searchTag = fmt.Sprintf("  🔍 \"%s\" (%d matches)  Esc:clear n/N:next/prev", m.searchQuery, matchCount)
	}
	stats := statsStyle.Width(w).Render(fmt.Sprintf(
		"  📡 Sessions: %d   │   Actions: %d   │   Alerts: %d   │   /: search  ↑↓: scroll  Enter: detail  tab: switch%s%s",
		m.sessions,
		m.engine.ActionCount(),
		m.engine.FindingCount(),
		followTag,
		searchTag,
	))

	// Split panels
	leftW := w * 2 / 3
	rightW := w - leftW - 2

	// === Actions panel (left) ===
	actHdr := "  Actions"
	if m.focusPanel == panelActions {
		actHdr = headerActiveStyle.Render("▸ Actions")
	} else {
		actHdr = headerStyle.Render("  Actions")
	}
	leftHeader := actHdr + "  " + headerStyle.Render(fmt.Sprintf("%-14s %-8s %-25s %s", "Time", "Session", "Tool", "Target"))

	actionOff := clampOffset(m.actionOffset, len(m.actions), vp)
	endIdx := actionOff + vp
	if endIdx > len(m.actions) {
		endIdx = len(m.actions)
	}
	var leftRows []string
	for i := actionOff; i < endIdx; i++ {
		row := m.actions[i]
		tool := row.tool
		if len(tool) > 25 {
			tool = tool[:25]
		}
		target := row.target
		maxTarget := leftW - 54
		if maxTarget < 10 {
			maxTarget = 10
		}
		if len(target) > maxTarget {
			target = target[:maxTarget]
		}
		prefix := "  "
		if i == m.actionCursor && m.focusPanel == panelActions {
			prefix = "▸ "
		}
		expandMark := " "
		if row.expanded {
			expandMark = "▾"
		} else if len(row.detail) > 0 {
			expandMark = "▸"
		}
		line := fmt.Sprintf("%s%s %-14s %-8s %-25s %s", prefix, expandMark, row.time, row.sessionID, tool, target)
		if i == m.actionCursor && m.focusPanel == panelActions {
			leftRows = append(leftRows, rowHighlightStyle.Render(line))
		} else if m.searchQuery != "" && isMatchedRow(m.filterIndicesActions, i) {
			leftRows = append(leftRows, rowMatchStyle.Render(line))
		} else {
			leftRows = append(leftRows, rowStyle.Render(line))
		}
		// Render expanded detail
		if row.expanded && len(row.detail) > 0 {
			for _, dl := range formatDetail(row.tool, row.detail) {
				leftRows = append(leftRows, detailStyle.Render("    │ "+dl))
			}
		}
	}
	// Pad empty lines to fill viewport
	for len(leftRows) < vp {
		leftRows = append(leftRows, "")
	}
	// Scroll indicator
	leftScroll := scrollIndicator(actionOff, len(m.actions), vp)

	leftContent := leftHeader + "\n" + strings.Join(leftRows, "\n") + "\n" + scrollInfoStyle.Render(leftScroll)

	// === Findings panel (right) ===
	fHdr := "  🛡 Alerts"
	if m.focusPanel == panelFindings {
		fHdr = headerActiveStyle.Render("▸ 🛡 Alerts")
	} else {
		fHdr = headerStyle.Render("  🛡 Alerts")
	}

	findingOff := clampOffset(m.findingOffset, len(m.findings), vp)
	fEndIdx := findingOff + vp
	if fEndIdx > len(m.findings) {
		fEndIdx = len(m.findings)
	}
	var rightRows []string
	for i := findingOff; i < fEndIdx; i++ {
		row := m.findings[i]
		evidence := row.evidence
		maxEvidence := rightW - 25
		if maxEvidence < 10 {
			maxEvidence = 10
		}
		if len(evidence) > maxEvidence {
			evidence = evidence[:maxEvidence]
		}
		prefix := "  "
		if i == m.findingCursor && m.focusPanel == panelFindings {
			prefix = "▸ "
		}
		expandMark := " "
		if row.expanded {
			expandMark = "▾"
		} else if row.description != "" || row.mitre != "" {
			expandMark = "▸"
		}
		line := fmt.Sprintf("%s%s %s %s %-4s %s", prefix, expandMark, row.time, sevLabel(row.severity), row.ruleID, evidence)
		if i == m.findingCursor && m.focusPanel == panelFindings {
			rightRows = append(rightRows, rowHighlightStyle.Render(line))
		} else if m.searchQuery != "" && isMatchedRow(m.filterIndicesFindings, i) {
			rightRows = append(rightRows, rowMatchStyle.Render(line))
		} else {
			rightRows = append(rightRows, line)
		}
		// Render expanded detail
		if row.expanded {
			if row.description != "" {
				rightRows = append(rightRows, detailStyle.Render("    │ "+row.description))
			}
			if row.mitre != "" && strings.TrimSpace(row.mitre) != "" {
				rightRows = append(rightRows, detailStyle.Render("    │ MITRE: "+row.mitre))
			}
			if row.tool != "" {
				rightRows = append(rightRows, detailStyle.Render("    │ Tool: "+row.tool))
			}
		}
	}
	if len(rightRows) == 0 && len(m.findings) == 0 {
		rightRows = append(rightRows, "  (no alerts)")
	}
	for len(rightRows) < vp {
		rightRows = append(rightRows, "")
	}
	rightScroll := scrollIndicator(findingOff, len(m.findings), vp)

	rightContent := fHdr + "\n" + strings.Join(rightRows, "\n") + "\n" + scrollInfoStyle.Render(rightScroll)

	// Combine
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

	if w < 80 {
		return title + "\n" + stats + "\n" + leftContent + "\n\n" + rightContent + "\n"
	}

	// Side-by-side using lipgloss.JoinHorizontal
	leftBlock := lipgloss.NewStyle().Width(leftW).Render(leftContent)
	divBlock := lipgloss.NewStyle().Width(1).Render(strings.Repeat(divider+"\n", vp+2))
	rightBlock := lipgloss.NewStyle().Width(rightW).Render(rightContent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, divBlock, rightBlock)

	return title + "\n" + stats + "\n" + body + "\n"
}

// formatDetail extracts key fields from action arguments for display.
func formatDetail(tool string, args map[string]interface{}) []string {
	var lines []string
	add := func(label string, val interface{}) {
		if val == nil {
			return
		}
		s := fmt.Sprintf("%v", val)
		if len(s) > 120 {
			s = s[:120] + "..."
		}
		lines = append(lines, label+": "+s)
	}

	switch tool {
	case "read_file":
		add("File", args["filePath"])
		add("Lines", fmt.Sprintf("%v–%v", args["startLine"], args["endLine"]))
	case "create_file":
		add("File", args["filePath"])
		if c, ok := args["content"].(string); ok {
			if len(c) > 80 {
				c = c[:80] + "..."
			}
			add("Content", c)
		}
	case "replace_string_in_file", "multi_replace_string_in_file":
		add("File", args["filePath"])
		if os, ok := args["oldString"].(string); ok {
			if len(os) > 80 {
				os = os[:80] + "..."
			}
			add("Old", os)
		}
		if ns, ok := args["newString"].(string); ok {
			if len(ns) > 80 {
				ns = ns[:80] + "..."
			}
			add("New", ns)
		}
	case "run_in_terminal", "send_to_terminal":
		add("Command", args["command"])
		add("Goal", args["goal"])
	case "fetch_webpage":
		add("URLs", args["urls"])
		add("Query", args["query"])
	case "grep_search", "semantic_search":
		add("Query", args["query"])
		add("Pattern", args["includePattern"])
	case "list_dir":
		add("Path", args["path"])
	default:
		for k, v := range args {
			add(k, v)
			if len(lines) >= 5 {
				break
			}
		}
	}
	return lines
}

// scrollIndicator returns a line like "  ↕ 1-25 of 142"
func scrollIndicator(offset, total, viewportH int) string {
	if total == 0 {
		return "  (empty)"
	}
	start := offset + 1
	end := offset + viewportH
	if end > total {
		end = total
	}
	return fmt.Sprintf("  ↕ %d–%d of %d", start, end, total)
}

// Commands

func listenActions(src watcher.EventSource) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-src.Actions()
		if !ok {
			return replayDoneMsg{}
		}
		return actionMsg{sessionID: ev.SessionID, action: ev.Action}
	}
}

func listenFindings(src watcher.EventSource) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-src.Findings()
		if !ok {
			return replayDoneMsg{}
		}
		return findingMsg{sessionID: ev.SessionID, finding: ev.Finding}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
