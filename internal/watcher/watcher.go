package watcher

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/aigaze-sec/aigaze/internal/discover"
	"github.com/aigaze-sec/aigaze/internal/engine"
	"github.com/aigaze-sec/aigaze/internal/parser"
)

// ActionEvent is emitted when a new action is detected.
type ActionEvent struct {
	SessionID string
	Action    parser.Action
}

// FindingEvent is emitted when a new security finding is detected.
type FindingEvent struct {
	SessionID string
	Finding   engine.Finding
}

// WatchEngine monitors transcript files for real-time changes.
type WatchEngine struct {
	ActionCh  chan ActionEvent
	FindingCh chan FindingEvent

	workspaceRoots []string
	watcher        *fsnotify.Watcher
	offsets        map[string]int64 // file path -> last read offset
	mu             sync.Mutex
	done           chan struct{}

	actionCount  int
	findingCount int
}

// New creates a new WatchEngine.
func New(workspaceRoots []string) (*WatchEngine, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &WatchEngine{
		ActionCh:       make(chan ActionEvent, 256),
		FindingCh:      make(chan FindingEvent, 256),
		workspaceRoots: workspaceRoots,
		watcher:        w,
		offsets:        make(map[string]int64),
		done:           make(chan struct{}),
	}, nil
}

// AddDirectory adds a directory to watch.
func (we *WatchEngine) AddDirectory(dir string) error {
	return we.watcher.Add(dir)
}

// AutoDiscover discovers and watches all known transcript directories.
func (we *WatchEngine) AutoDiscover() []discover.Location {
	locations := discover.Discover()
	for _, loc := range locations {
		_ = we.watcher.Add(loc.Path)
	}
	return locations
}

// Start begins watching for file changes. Blocks until Stop() is called.
func (we *WatchEngine) Start() {
	for {
		select {
		case event, ok := <-we.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) && strings.HasSuffix(event.Name, ".jsonl") {
				we.processFile(event.Name)
			}
		case _, ok := <-we.watcher.Errors:
			if !ok {
				return
			}
		case <-we.done:
			return
		}
	}
}

// Stop stops the watcher.
func (we *WatchEngine) Stop() {
	close(we.done)
	we.watcher.Close()
}

// Actions returns the action event channel (implements EventSource).
func (we *WatchEngine) Actions() <-chan ActionEvent { return we.ActionCh }

// Findings returns the finding event channel (implements EventSource).
func (we *WatchEngine) Findings() <-chan FindingEvent { return we.FindingCh }

// ActionCount returns the total actions processed.
func (we *WatchEngine) ActionCount() int {
	we.mu.Lock()
	defer we.mu.Unlock()
	return we.actionCount
}

// FindingCount returns the total findings detected.
func (we *WatchEngine) FindingCount() int {
	we.mu.Lock()
	defer we.mu.Unlock()
	return we.findingCount
}

func (we *WatchEngine) processFile(path string) {
	we.mu.Lock()
	offset := we.offsets[path]
	we.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil || info.Size() <= offset {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return
	}

	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 10*1024*1024)

	var newActions []parser.Action

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}

		evType, _ := ev["type"].(string)
		if evType != "assistant.message" {
			continue
		}
		data, _ := ev["data"].(map[string]interface{})
		if data == nil {
			continue
		}
		timestamp, _ := ev["timestamp"].(string)

		toolRequests, ok := data["toolRequests"].([]interface{})
		if !ok {
			continue
		}

		for _, raw := range toolRequests {
			tr, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			toolName, _ := tr["name"].(string)
			toolCallID, _ := tr["toolCallId"].(string)
			argsRaw, _ := tr["arguments"].(string)

			args := make(map[string]interface{})
			if argsRaw != "" {
				_ = json.Unmarshal([]byte(argsRaw), &args)
			}

			action := parser.Action{
				Tool:       toolName,
				ToolCallID: toolCallID,
				Target:     parser.ClassifyTool(toolName), // reuse for target extraction
				Content:    argsRaw,
				Arguments:  args,
				Timestamp:  timestamp,
				TurnID:     "",
				ActionType: parser.ClassifyTool(toolName),
			}
			// Fix target
			action.Target = extractTarget(toolName, args)

			newActions = append(newActions, action)
		}
	}

	// Update offset
	newOffset, _ := f.Seek(0, 1)
	we.mu.Lock()
	we.offsets[path] = newOffset
	we.mu.Unlock()

	// Process actions
	for _, action := range newActions {
		we.mu.Lock()
		we.actionCount++
		we.mu.Unlock()

		we.ActionCh <- ActionEvent{SessionID: sessionID, Action: action}

		// Quick scan: single-action session
		miniSession := &parser.Session{
			SessionID: sessionID,
			Actions:   []parser.Action{action},
			Turns: []parser.Turn{
				{Actions: []parser.Action{action}},
			},
		}
		result := engine.ScanSession(miniSession, we.workspaceRoots)
		for _, finding := range result.Findings {
			if !finding.Suppressed {
				we.mu.Lock()
				we.findingCount++
				we.mu.Unlock()
				we.FindingCh <- FindingEvent{SessionID: sessionID, Finding: finding}
			}
		}
	}
}

func extractTarget(tool string, args map[string]interface{}) string {
	switch tool {
	case "read_file", "create_file", "replace_string_in_file":
		if fp, ok := args["filePath"].(string); ok {
			return fp
		}
	case "run_in_terminal", "send_to_terminal":
		if cmd, ok := args["command"].(string); ok {
			if len(cmd) > 80 {
				return cmd[:80]
			}
			return cmd
		}
	case "fetch_webpage":
		if urls, ok := args["urls"].([]interface{}); ok && len(urls) > 0 {
			if u, ok := urls[0].(string); ok {
				return u
			}
		}
	case "grep_search", "semantic_search":
		if q, ok := args["query"].(string); ok {
			if len(q) > 80 {
				return q[:80]
			}
			return q
		}
	case "list_dir":
		if p, ok := args["path"].(string); ok {
			return p
		}
	}
	return ""
}
