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
}

type findingRow struct {
	time     string
	severity string
	ruleID   string
	evidence string
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
				m.actionOffset++
				m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
			} else {
				m.findingOffset++
				m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
			}
			return m, nil
		case "k", "up":
			m.autoFollow = false
			if m.focusPanel == panelActions {
				m.actionOffset--
				m.actionOffset = clampOffset(m.actionOffset, len(m.actions), vp)
			} else {
				m.findingOffset--
				m.findingOffset = clampOffset(m.findingOffset, len(m.findings), vp)
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
			} else {
				m.findingOffset = clampOffset(len(m.findings), len(m.findings), vp)
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
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case actionMsg:
		now := time.Now().Format("15:04:05")
		target := msg.action.Target
		if len(target) > 80 {
			target = target[:80]
		}
		m.actions = append(m.actions, actionRow{
			time:      now,
			sessionID: msg.sessionID,
			tool:      msg.action.Tool,
			target:    target,
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
		}
		return m, listenActions(m.engine)

	case replayDoneMsg:
		return m, nil

	case findingMsg:
		now := time.Now().Format("15:04:05")
		evidence := msg.finding.Evidence
		if len(evidence) > 60 {
			evidence = evidence[:60]
		}
		m.findings = append(m.findings, findingRow{
			time:     now,
			severity: msg.finding.Severity,
			ruleID:   msg.finding.RuleID,
			evidence: evidence,
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
		}
		return m, listenFindings(m.engine)

	case tickMsg:
		return m, tickCmd()
	}

	return m, nil
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
	stats := statsStyle.Width(w).Render(fmt.Sprintf(
		"  📡 Sessions: %d   │   Actions: %d   │   Alerts: %d   │   ↑↓/jk: scroll  tab: switch  f: follow  q: quit  c: clear%s",
		m.sessions,
		m.engine.ActionCount(),
		m.engine.FindingCount(),
		followTag,
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
	leftHeader := actHdr + "  " + headerStyle.Render(fmt.Sprintf("%-8s %-8s %-25s %s", "Time", "Session", "Tool", "Target"))

	actionOff := clampOffset(m.actionOffset, len(m.actions), vp)
	endIdx := actionOff + vp
	if endIdx > len(m.actions) {
		endIdx = len(m.actions)
	}
	var leftRows []string
	for _, row := range m.actions[actionOff:endIdx] {
		tool := row.tool
		if len(tool) > 25 {
			tool = tool[:25]
		}
		target := row.target
		maxTarget := leftW - 48
		if maxTarget < 10 {
			maxTarget = 10
		}
		if len(target) > maxTarget {
			target = target[:maxTarget]
		}
		leftRows = append(leftRows, rowStyle.Render(fmt.Sprintf(
			"  %-8s %-8s %-25s %s", row.time, row.sessionID, tool, target,
		)))
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
	for _, row := range m.findings[findingOff:fEndIdx] {
		evidence := row.evidence
		maxEvidence := rightW - 25
		if maxEvidence < 10 {
			maxEvidence = 10
		}
		if len(evidence) > maxEvidence {
			evidence = evidence[:maxEvidence]
		}
		rightRows = append(rightRows, fmt.Sprintf(
			"  %s %s %-4s %s", row.time, sevLabel(row.severity), row.ruleID, evidence,
		))
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
