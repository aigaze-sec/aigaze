package watcher

import (
	"sync"
	"time"

	"github.com/aigaze-sec/aigaze/internal/engine"
	"github.com/aigaze-sec/aigaze/internal/parser"
)

// SessionMeta describes a loaded session for the /api/sessions endpoint.
type SessionMeta struct {
	SessionID string `json:"session_id"`
	Model     string `json:"model,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	Actions   int    `json:"actions"`
	Findings  int    `json:"findings"`
}

// MultiReplayEngine replays multiple sessions through the same channels.
type MultiReplayEngine struct {
	actionCh  chan ActionEvent
	findingCh chan FindingEvent

	workspaceRoots []string
	sessions       []*parser.Session
	delayMs        int
	done           chan struct{}

	mu           sync.Mutex
	actionCount  int
	findingCount int
	sessionMetas []SessionMeta
}

// NewMultiReplay creates a MultiReplayEngine from multiple parsed sessions.
func NewMultiReplay(sessions []*parser.Session, workspaceRoots []string, delayMs int) *MultiReplayEngine {
	return &MultiReplayEngine{
		actionCh:       make(chan ActionEvent, 256),
		findingCh:      make(chan FindingEvent, 256),
		workspaceRoots: workspaceRoots,
		sessions:       sessions,
		delayMs:        delayMs,
		done:           make(chan struct{}),
	}
}

func (m *MultiReplayEngine) Actions() <-chan ActionEvent  { return m.actionCh }
func (m *MultiReplayEngine) Findings() <-chan FindingEvent { return m.findingCh }

func (m *MultiReplayEngine) ActionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.actionCount
}

func (m *MultiReplayEngine) FindingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.findingCount
}

func (m *MultiReplayEngine) Stop() {
	select {
	case <-m.done:
	default:
		close(m.done)
	}
}

// Sessions returns metadata for all loaded sessions.
func (m *MultiReplayEngine) Sessions() []SessionMeta {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SessionMeta, len(m.sessionMetas))
	copy(out, m.sessionMetas)
	return out
}

// SessionCount returns the number of loaded sessions.
func (m *MultiReplayEngine) SessionCount() int {
	return len(m.sessions)
}

// Start replays all sessions sequentially. Call in a goroutine.
func (m *MultiReplayEngine) Start() {
	delay := time.Duration(m.delayMs) * time.Millisecond

	for _, sess := range m.sessions {
		sessionID := sess.SessionID

		result := engine.ScanSession(sess, m.workspaceRoots)

		// Build finding map per action
		type actionKey struct {
			tool      string
			timestamp string
		}
		actionIndex := make(map[actionKey]int)
		for i, a := range sess.Actions {
			k := actionKey{tool: a.Tool, timestamp: a.Timestamp}
			if _, exists := actionIndex[k]; !exists {
				actionIndex[k] = i
			}
		}
		findingsByAction := make(map[int][]engine.Finding)
		sessionFindings := 0
		for _, f := range result.Findings {
			if f.Suppressed {
				continue
			}
			sessionFindings++
			k := actionKey{tool: f.Tool, timestamp: f.Timestamp}
			if idx, ok := actionIndex[k]; ok {
				findingsByAction[idx] = append(findingsByAction[idx], f)
			} else if len(sess.Actions) > 0 {
				findingsByAction[0] = append(findingsByAction[0], f)
			}
		}

		// Record session metadata
		m.mu.Lock()
		m.sessionMetas = append(m.sessionMetas, SessionMeta{
			SessionID: sess.SessionID,
			Model:     sess.Model,
			StartTime: sess.StartTime,
			Actions:   len(sess.Actions),
			Findings:  sessionFindings,
		})
		m.mu.Unlock()

		// Replay actions
		for i, action := range sess.Actions {
			select {
			case <-m.done:
				return
			default:
			}

			m.mu.Lock()
			m.actionCount++
			m.mu.Unlock()

			m.actionCh <- ActionEvent{SessionID: sessionID, Action: action}

			if findings, ok := findingsByAction[i]; ok {
				for _, f := range findings {
					m.mu.Lock()
					m.findingCount++
					m.mu.Unlock()
					m.findingCh <- FindingEvent{SessionID: sessionID, Finding: f}
				}
			}

			if delay > 0 {
				select {
				case <-m.done:
					return
				case <-time.After(delay):
				}
			}
		}
	}
}
