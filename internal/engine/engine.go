package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aigaze-sec/aigaze/internal/parser"
)

// Rule defines a single detection rule.
type Rule struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	Severity             string   `json:"severity"`
	MITRETechnique       string   `json:"mitre_technique"`
	MITREName            string   `json:"mitre_name"`
	ActionTypes          []string `json:"action_types"`
	Patterns             []*regexp.Regexp
	CheckType            string // "pattern", "workspace_boundary", "url_allowlist"
	SuppressPlaceholders bool   // if true, suppress known placeholder matches
}

// Finding is a single security finding from rule evaluation.
type Finding struct {
	RuleID         string `json:"rule_id"`
	RuleName       string `json:"rule_name"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	MITRETechnique string `json:"mitre_technique"`
	MITREName      string `json:"mitre_name"`
	Tool           string `json:"tool"`
	ActionType     string `json:"action_type"`
	Target         string `json:"target"`
	Evidence       string `json:"evidence"`
	Timestamp      string `json:"timestamp"`
	TurnID         string `json:"turn_id"`
	Suppressed     bool   `json:"suppressed"`
	SuppressReason string `json:"suppress_reason,omitempty"`
}

// ScanResult contains all findings from scanning a session.
type ScanResult struct {
	SessionID string    `json:"session_id"`
	Model     string    `json:"model"`
	Findings  []Finding `json:"findings"`
}

// FindingCount returns the number of non-suppressed findings.
func (r *ScanResult) FindingCount() int {
	count := 0
	for _, f := range r.Findings {
		if !f.Suppressed {
			count++
		}
	}
	return count
}

// SuppressedCount returns the number of suppressed findings.
func (r *ScanResult) SuppressedCount() int {
	count := 0
	for _, f := range r.Findings {
		if f.Suppressed {
			count++
		}
	}
	return count
}

// Known placeholder secrets that should not trigger alerts.
var placeholders = []string{
	"AKIAIOSFODNN7EXAMPLE",
	"AKIAI44QH8DHBEXAMPLE",
	"your_api_key_here",
	"sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
}

// DefaultRules returns the built-in L1 detection rules.
func DefaultRules() []Rule {
	return []Rule{
		{
			ID:             "R1",
			Name:           "Credential File Access",
			Description:    "AI agent accessed credential or sensitive configuration files",
			Severity:       "HIGH",
			MITRETechnique: "T1552.001",
			MITREName:      "Unsecured Credentials: Credentials In Files",
			ActionTypes:    []string{"file_read", "dir_list"},
			Patterns: compile(
				`(?i)(\.ssh|\.aws|\.env|credentials|\.kube/config|\.docker/config|id_rsa|id_ed25519|\.pem$|/etc/passwd|/etc/shadow|\.netrc|\.pgpass)`,
			),
			CheckType: "pattern",
		},
		{
			ID:             "R2",
			Name:           "Dangerous Terminal Command",
			Description:    "AI agent executed potentially dangerous terminal commands",
			Severity:       "CRITICAL",
			MITRETechnique: "T1059",
			MITREName:      "Command and Scripting Interpreter",
			ActionTypes:    []string{"terminal_exec", "terminal_input"},
			Patterns: compile(
				`rm\s+-rf\s+/`,
				`chmod\s+777`,
				`curl\s+.*\|\s*sh`,
				`curl\s+.*\|\s*bash`,
				`wget\s+.*\|\s*sh`,
				`eval\s*\(`,
				`base64\s+-d`,
				`nc\s+-l`,
				`ncat\s+`,
				`dd\s+if=`,
				`mkfs\.`,
				`:(){.*};:`,
			),
			CheckType: "pattern",
		},
		{
			ID:             "R3",
			Name:           "Hardcoded Secret in Code",
			Description:    "AI agent wrote code containing hardcoded secrets or API keys",
			Severity:       "CRITICAL",
			MITRETechnique: "T1552.001",
			MITREName:      "Unsecured Credentials: Credentials In Files",
			ActionTypes:    []string{"file_create", "file_edit"},
			Patterns: compile(
				`AKIA[0-9A-Z]{16}`,
				`ghp_[A-Za-z0-9]{36}`,
				`gho_[A-Za-z0-9]{36}`,
				`github_pat_[A-Za-z0-9_]{40,}`,
				`sk-[A-Za-z0-9]{20,}`,
				`xox[bpors]-[A-Za-z0-9\-]{10,}`,
				`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,
				`pypi-[A-Za-z0-9_\-]{50,}`,
			),
			CheckType: "pattern",
		},
		{
			ID:             "R4",
			Name:           "Out-of-Workspace File Access",
			Description:    "AI agent accessed files outside the current workspace",
			Severity:       "MEDIUM",
			MITRETechnique: "T1083",
			MITREName:      "File and Directory Discovery",
			ActionTypes:    []string{"file_read", "file_create", "file_edit", "dir_list"},
			CheckType:      "workspace_boundary",
		},
		{
			ID:             "R5",
			Name:           "External URL Access",
			Description:    "AI agent accessed external URLs",
			Severity:       "LOW",
			MITRETechnique: "T1071",
			MITREName:      "Application Layer Protocol",
			ActionTypes:    []string{"network_fetch"},
			CheckType:      "url_allowlist",
		},
	}
}

// ScanSession scans a session against all rules and returns findings.
func ScanSession(session *parser.Session, workspaceRoots []string) *ScanResult {
	rules, err := LoadAllRules()
	if err != nil {
		// Fallback to hardcoded defaults if YAML loading fails
		rules = DefaultRules()
	}
	_ = err
	return ScanSessionWithRules(session, workspaceRoots, rules)
}

// ScanSessionWithRules scans a session against the given rules and returns findings.
func ScanSessionWithRules(session *parser.Session, workspaceRoots []string, rules []Rule) *ScanResult {
	result := &ScanResult{
		SessionID: session.SessionID,
		Model:     session.Model,
	}

	for _, rule := range rules {
		switch rule.CheckType {
		case "pattern":
			checkPatterns(&rule, session, result)
		case "workspace_boundary":
			checkWorkspaceBoundary(&rule, session, workspaceRoots, result)
		case "url_allowlist":
			checkURLAllowlist(&rule, session, result)
		}
	}

	return result
}

func checkPatterns(rule *Rule, session *parser.Session, result *ScanResult) {
	actionTypeSet := make(map[string]bool)
	for _, at := range rule.ActionTypes {
		actionTypeSet[at] = true
	}

	for _, action := range session.Actions {
		if !actionTypeSet[action.ActionType] {
			continue
		}

		// Build text to match against
		matchText := action.Target + " " + action.Content
		if cmd, ok := action.Arguments["command"].(string); ok {
			matchText += " " + cmd
		}

		for _, pat := range rule.Patterns {
			if loc := pat.FindStringIndex(matchText); loc != nil {
				matched := matchText[loc[0]:loc[1]]
				evidence := matched
				if len(evidence) > 80 {
					evidence = evidence[:80]
				}

				// Check for placeholder suppression
				suppressed := false
				suppressReason := ""
				if rule.SuppressPlaceholders {
					for _, ph := range placeholders {
						if strings.Contains(matched, ph) {
							suppressed = true
							suppressReason = "known placeholder"
							break
						}
					}
					if strings.Contains(strings.ToUpper(matched), "EXAMPLE") ||
						strings.Contains(strings.ToLower(matched), "xxx") {
						suppressed = true
						suppressReason = "placeholder pattern"
					}
				}

				result.Findings = append(result.Findings, Finding{
					RuleID:         rule.ID,
					RuleName:       rule.Name,
					Severity:       rule.Severity,
					Description:    rule.Description,
					MITRETechnique: rule.MITRETechnique,
					MITREName:      rule.MITREName,
					Tool:           action.Tool,
					ActionType:     action.ActionType,
					Target:         action.Target,
					Evidence:       evidence,
					Timestamp:      action.Timestamp,
					TurnID:         action.TurnID,
					Suppressed:     suppressed,
					SuppressReason: suppressReason,
				})
				break // one finding per action per rule
			}
		}
	}
}

func checkWorkspaceBoundary(rule *Rule, session *parser.Session, roots []string, result *ScanResult) {
	if len(roots) == 0 {
		return
	}

	actionTypeSet := make(map[string]bool)
	for _, at := range rule.ActionTypes {
		actionTypeSet[at] = true
	}

	for _, action := range session.Actions {
		if !actionTypeSet[action.ActionType] {
			continue
		}
		target := action.Target
		if target == "" {
			continue
		}

		inWorkspace := false
		for _, root := range roots {
			if strings.HasPrefix(target, root) {
				inWorkspace = true
				break
			}
		}

		if !inWorkspace {
			result.Findings = append(result.Findings, Finding{
				RuleID:         rule.ID,
				RuleName:       rule.Name,
				Severity:       rule.Severity,
				Description:    rule.Description,
				MITRETechnique: rule.MITRETechnique,
				MITREName:      rule.MITREName,
				Tool:           action.Tool,
				ActionType:     action.ActionType,
				Target:         action.Target,
				Evidence:       target,
				Timestamp:      action.Timestamp,
				TurnID:         action.TurnID,
			})
		}
	}
}

func checkURLAllowlist(rule *Rule, session *parser.Session, result *ScanResult) {
	localPrefixes := []string{
		"http://localhost", "https://localhost",
		"http://127.0.0.1", "https://127.0.0.1",
		"http://0.0.0.0", "https://0.0.0.0",
		"http://[::1]", "https://[::1]",
	}

	for _, action := range session.Actions {
		if action.ActionType != "network_fetch" {
			continue
		}
		target := action.Target
		if target == "" {
			continue
		}

		isLocal := false
		for _, prefix := range localPrefixes {
			if strings.HasPrefix(target, prefix) {
				isLocal = true
				break
			}
		}

		if !isLocal {
			result.Findings = append(result.Findings, Finding{
				RuleID:         rule.ID,
				RuleName:       rule.Name,
				Severity:       rule.Severity,
				Description:    fmt.Sprintf("External URL accessed: %s", target),
				MITRETechnique: rule.MITRETechnique,
				MITREName:      rule.MITREName,
				Tool:           action.Tool,
				ActionType:     action.ActionType,
				Target:         action.Target,
				Evidence:       target,
				Timestamp:      action.Timestamp,
				TurnID:         action.TurnID,
			})
		}
	}
}

func compile(patterns ...string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}
