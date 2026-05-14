package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Action represents a single tool-call action from a transcript.
type Action struct {
	Tool       string                 `json:"tool"`
	ToolCallID string                 `json:"tool_call_id"`
	Target     string                 `json:"target"`
	Content    string                 `json:"content"`
	Arguments  map[string]interface{} `json:"arguments"`
	Timestamp  string                 `json:"timestamp"`
	TurnID     string                 `json:"turn_id"`
	ActionType string                 `json:"action_type"`
	Success    *bool                  `json:"success,omitempty"`
}

// Turn represents a single assistant turn containing zero or more actions.
type Turn struct {
	TurnID    string   `json:"turn_id"`
	Timestamp string   `json:"timestamp"`
	Actions   []Action `json:"actions"`
}

// Session represents a fully parsed transcript session.
type Session struct {
	SessionID  string   `json:"session_id"`
	Model      string   `json:"model"`
	CopilotVer string   `json:"copilot_version"`
	VSCodeVer  string   `json:"vscode_version"`
	StartTime  string   `json:"start_time"`
	Actions    []Action `json:"actions"`
	Turns      []Turn   `json:"turns"`
}

// ParseTranscript reads a .jsonl transcript file and returns a Session.
func ParseTranscript(path string) (*Session, error) {
	sessions, err := ParseMultiSession(path)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return &Session{SessionID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}, nil
	}
	// Merge all sessions into one for backward compatibility
	merged := sessions[0]
	for i := 1; i < len(sessions); i++ {
		merged.Actions = append(merged.Actions, sessions[i].Actions...)
		merged.Turns = append(merged.Turns, sessions[i].Turns...)
	}
	return merged, nil
}

// ParseMultiSession reads a .jsonl transcript file and returns one Session per
// session.start event found. If no session.start is found, the entire file is
// returned as a single session.
func ParseMultiSession(path string) ([]*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	var sessions []*Session
	var sess *Session
	var currentTurn *Turn
	sessionIdx := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB line buffer

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
		data, _ := ev["data"].(map[string]interface{})
		timestamp, _ := ev["timestamp"].(string)

		switch evType {
		case "session.start":
			// Close previous session's unclosed turn
			if sess != nil && currentTurn != nil {
				sess.Turns = append(sess.Turns, *currentTurn)
				currentTurn = nil
			}
			// Start new session
			sessionIdx++
			sid := fileID
			if sessionIdx > 1 {
				sid = fmt.Sprintf("%s#%d", fileID, sessionIdx)
			}
			sess = &Session{SessionID: sid}
			sessions = append(sessions, sess)
			if data != nil {
				sess.Model, _ = data["model"].(string)
				sess.StartTime = timestamp
				if ext, ok := data["extensions"].(map[string]interface{}); ok {
					sess.CopilotVer, _ = ext["copilotVersion"].(string)
					sess.VSCodeVer, _ = ext["vscodeVersion"].(string)
				}
			}

		case "assistant.turn_start":
			// Ensure we have a session
			if sess == nil {
				sess = &Session{SessionID: fileID}
				sessions = append(sessions, sess)
			}
			turnID := ""
			if data != nil {
				turnID, _ = data["turnId"].(string)
			}
			currentTurn = &Turn{TurnID: turnID, Timestamp: timestamp}

		case "assistant.message":
			if data == nil || currentTurn == nil {
				continue
			}
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

				target := extractTarget(toolName, args)
				content := argsRaw

				action := Action{
					Tool:       toolName,
					ToolCallID: toolCallID,
					Target:     target,
					Content:    content,
					Arguments:  args,
					Timestamp:  timestamp,
					TurnID:     currentTurn.TurnID,
					ActionType: ClassifyTool(toolName),
				}

				currentTurn.Actions = append(currentTurn.Actions, action)
				sess.Actions = append(sess.Actions, action)
			}

		case "tool.execution_complete":
			if data == nil || currentTurn == nil {
				continue
			}
			tcID, _ := data["toolCallId"].(string)
			success, _ := data["success"].(bool)
			for i := range currentTurn.Actions {
				if currentTurn.Actions[i].ToolCallID == tcID {
					currentTurn.Actions[i].Success = &success
					// Also update in session.Actions
					for j := range sess.Actions {
						if sess.Actions[j].ToolCallID == tcID {
							sess.Actions[j].Success = &success
							break
						}
					}
					break
				}
			}

		case "assistant.turn_end":
			if currentTurn != nil {
				sess.Turns = append(sess.Turns, *currentTurn)
				currentTurn = nil
			}
		}
	}

	// If there's an unclosed turn
	if sess != nil && currentTurn != nil {
		sess.Turns = append(sess.Turns, *currentTurn)
	}

	// If no session.start was found, return file as single session
	if len(sessions) == 0 {
		sessions = append(sessions, &Session{SessionID: fileID})
	}

	return sessions, scanner.Err()
}

func extractTarget(tool string, args map[string]interface{}) string {
	switch tool {
	case "read_file", "create_file", "replace_string_in_file", "multi_replace_string_in_file":
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
	case "file_search":
		if q, ok := args["query"].(string); ok {
			return q
		}
	case "list_dir":
		if p, ok := args["path"].(string); ok {
			return p
		}
	}
	return ""
}

// ClassifyTool maps a tool name to a semantic action type.
func ClassifyTool(tool string) string {
	switch tool {
	case "read_file":
		return "file_read"
	case "create_file":
		return "file_create"
	case "replace_string_in_file", "multi_replace_string_in_file":
		return "file_edit"
	case "list_dir":
		return "dir_list"
	case "file_search":
		return "file_search"
	case "grep_search":
		return "text_search"
	case "semantic_search":
		return "semantic_search"
	case "run_in_terminal":
		return "terminal_exec"
	case "send_to_terminal":
		return "terminal_input"
	case "get_terminal_output":
		return "terminal_output"
	case "fetch_webpage":
		return "network_fetch"
	case "manage_todo_list":
		return "task_manage"
	case "memory":
		return "memory_access"
	case "vscode_askQuestions":
		return "user_interact"
	case "runSubagent":
		return "agent_spawn"
	default:
		return "unknown"
	}
}
