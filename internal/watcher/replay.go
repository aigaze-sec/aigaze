package watcher

import (
	"sync"
	"time"

	"github.com/aigaze-sec/aigaze/internal/engine"
	"github.com/aigaze-sec/aigaze/internal/parser"
)

// ReplayEngine replays a parsed transcript through the same channels as
// WatchEngine, enabling offline analysis in the TUI.
type ReplayEngine struct {
	actionCh  chan ActionEvent
	findingCh chan FindingEvent

	workspaceRoots []string
	session        *parser.Session
	delayMs        int // milliseconds between events, 0 = instant
	done           chan struct{}

	mu           sync.Mutex
	actionCount  int
	findingCount int
}

// NewReplay creates a ReplayEngine from a parsed session.
// delayMs controls replay speed: 0 means instant (dump all at once).
func NewReplay(session *parser.Session, workspaceRoots []string, delayMs int) *ReplayEngine {
	return &ReplayEngine{
		actionCh:       make(chan ActionEvent, 256),
		findingCh:      make(chan FindingEvent, 256),
		workspaceRoots: workspaceRoots,
		session:        session,
		delayMs:        delayMs,
		done:           make(chan struct{}),
	}
}

// Actions returns the action event channel (implements EventSource).
func (re *ReplayEngine) Actions() <-chan ActionEvent { return re.actionCh }

// Findings returns the finding event channel (implements EventSource).
func (re *ReplayEngine) Findings() <-chan FindingEvent { return re.findingCh }

// ActionCount returns the total replayed actions so far.
func (re *ReplayEngine) ActionCount() int {
	re.mu.Lock()
	defer re.mu.Unlock()
	return re.actionCount
}

// FindingCount returns the total findings so far.
func (re *ReplayEngine) FindingCount() int {
	re.mu.Lock()
	defer re.mu.Unlock()
	return re.findingCount
}

// Stop signals the replay to stop.
func (re *ReplayEngine) Stop() {
	select {
	case <-re.done:
	default:
		close(re.done)
	}
}

// SessionCount returns 1 (replaying a single file).
func (re *ReplayEngine) SessionCount() int { return 1 }

// Start begins replaying actions into the channels. Call in a goroutine.
// When replay finishes, channels remain open so the TUI stays up for review.
func (re *ReplayEngine) Start() {
	sessionID := re.session.SessionID

	// Scan the full session once for findings
	result := engine.ScanSession(re.session, re.workspaceRoots)

	// Build a map from action tool+target to findings for ordered emission.
	// We associate each finding with the first matching action index.
	type actionKey struct {
		tool      string
		timestamp string
	}
	actionIndex := make(map[actionKey]int)
	for i, a := range re.session.Actions {
		k := actionKey{tool: a.Tool, timestamp: a.Timestamp}
		if _, exists := actionIndex[k]; !exists {
			actionIndex[k] = i
		}
	}

	findingsByAction := make(map[int][]engine.Finding)
	for _, f := range result.Findings {
		if f.Suppressed {
			continue
		}
		// Best-effort match by tool+timestamp
		k := actionKey{tool: f.Tool, timestamp: f.Timestamp}
		if idx, ok := actionIndex[k]; ok {
			findingsByAction[idx] = append(findingsByAction[idx], f)
		} else {
			// Fallback: emit with first action
			findingsByAction[0] = append(findingsByAction[0], f)
		}
	}

	delay := time.Duration(re.delayMs) * time.Millisecond

	for i, action := range re.session.Actions {
		select {
		case <-re.done:
			return
		default:
		}

		re.mu.Lock()
		re.actionCount++
		re.mu.Unlock()

		re.actionCh <- ActionEvent{SessionID: sessionID, Action: action}

		// Emit any findings associated with this action
		if findings, ok := findingsByAction[i]; ok {
			for _, f := range findings {
				re.mu.Lock()
				re.findingCount++
				re.mu.Unlock()
				re.findingCh <- FindingEvent{SessionID: sessionID, Finding: f}
			}
		}

		if delay > 0 {
			select {
			case <-re.done:
				return
			case <-time.After(delay):
			}
		}
	}
}
