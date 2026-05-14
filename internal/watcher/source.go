package watcher

// EventSource is the interface consumed by the TUI.
// Both WatchEngine (real-time) and ReplayEngine (offline) implement it.
type EventSource interface {
	Actions() <-chan ActionEvent
	Findings() <-chan FindingEvent
	ActionCount() int
	FindingCount() int
	Stop()
}

// SessionLister is optionally implemented by EventSource implementations
// that can enumerate their sessions (e.g. MultiReplayEngine).
type SessionLister interface {
	Sessions() []SessionMeta
}
