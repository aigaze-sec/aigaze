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

const maxRows = 100

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

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	sevCRIT   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("🔴 CRIT")
	sevHIGH   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")).Render("🟠 HIGH")
	sevMED    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("⚠  MED")
	sevLOW    = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("ℹ  LOW")
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
	engine      watcher.EventSource
	sessions    int
	actions     []actionRow
	findings    []findingRow
	width       int
	height      int
	quitting    bool
	replayMode  bool
}

// NewModel creates a new TUI model from any EventSource.
func NewModel(src watcher.EventSource, sessionCount int, replayMode bool) Model {
	return Model{
		engine:     src,
		sessions:   sessionCount,
		replayMode: replayMode,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenActions(m.engine),
		listenFindings(m.engine),
		tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.engine.Stop()
			return m, tea.Quit
		case "c":
			m.actions = nil
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case actionMsg:
		now := time.Now().Format("15:04:05")
		target := msg.action.Target
		if len(target) > 50 {
			target = target[:50]
		}
		m.actions = append(m.actions, actionRow{
			time:      now,
			sessionID: msg.sessionID,
			tool:      msg.action.Tool,
			target:    target,
		})
		if len(m.actions) > maxRows {
			m.actions = m.actions[len(m.actions)-maxRows:]
		}
		return m, listenActions(m.engine)

	case replayDoneMsg:
		// replay finished — keep TUI open for review
		return m, nil

	case findingMsg:
		now := time.Now().Format("15:04:05")
		evidence := msg.finding.Evidence
		if len(evidence) > 40 {
			evidence = evidence[:40]
		}
		m.findings = append(m.findings, findingRow{
			time:     now,
			severity: msg.finding.Severity,
			ruleID:   msg.finding.RuleID,
			evidence: evidence,
		})
		if len(m.findings) > maxRows {
			m.findings = m.findings[len(m.findings)-maxRows:]
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
	h := m.height
	if h == 0 {
		h = 40
	}

	// Title
	var titleText string
	if m.replayMode {
		titleText = "  AIGaze Watch — Offline Replay"
	} else {
		titleText = "  AIGaze Watch — Real-time AI Agent Action Monitor"
	}
	title := titleStyle.Render(titleText)

	// Stats bar
	stats := statsStyle.Width(w).Render(fmt.Sprintf(
		"  📡 Sessions: %d   │   Actions: %d   │   Alerts: %d   │   q: quit  c: clear",
		m.sessions,
		m.engine.ActionCount(),
		m.engine.FindingCount(),
	))

	// Split panels
	leftW := w * 2 / 3
	rightW := w - leftW - 1 // -1 for divider

	// Actions panel (left)
	leftHeader := headerStyle.Render(fmt.Sprintf("  %-8s %-8s %-25s %s", "Time", "Session", "Tool", "Target"))
	var leftRows []string
	startIdx := 0
	availableRows := h - 7
	if len(m.actions) > availableRows {
		startIdx = len(m.actions) - availableRows
	}
	for _, row := range m.actions[startIdx:] {
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
	leftContent := leftHeader + "\n" + strings.Join(leftRows, "\n")

	// Findings panel (right)
	rightHeader := headerStyle.Render(fmt.Sprintf("  🛡 Alerts"))
	var rightRows []string
	fStartIdx := 0
	if len(m.findings) > availableRows {
		fStartIdx = len(m.findings) - availableRows
	}
	for _, row := range m.findings[fStartIdx:] {
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
	if len(rightRows) == 0 {
		rightRows = append(rightRows, "  (no alerts)")
	}
	rightContent := rightHeader + "\n" + strings.Join(rightRows, "\n")

	// Combine
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

	// Simple side-by-side (just stack vertically for now if terminal is narrow)
	if w < 80 {
		return title + "\n" + stats + "\n\n" + leftContent + "\n\n" + rightContent + "\n"
	}

	return title + "\n" + stats + "\n\n" + leftContent + "\n\n" + divider + "\n" + rightContent + "\n"
}

// Commands

type replayDoneMsg struct{}

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
