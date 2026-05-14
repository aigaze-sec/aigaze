package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aigaze-sec/aigaze/internal/watcher"
)

//go:embed static
var staticFiles embed.FS

// actionJSON is the JSON shape sent to the browser.
type actionJSON struct {
	Time       string                 `json:"time"`
	SessionID  string                 `json:"session_id"`
	Tool       string                 `json:"tool"`
	Target     string                 `json:"target"`
	ActionType string                 `json:"action_type,omitempty"`
	Detail     map[string]interface{} `json:"detail,omitempty"`
}

type findingJSON struct {
	Time        string `json:"time"`
	Severity    string `json:"severity"`
	RuleID      string `json:"rule_id"`
	RuleName    string `json:"rule_name"`
	Evidence    string `json:"evidence"`
	Description string `json:"description,omitempty"`
	MITRE       string `json:"mitre,omitempty"`
	Tool        string `json:"tool,omitempty"`
	Target      string `json:"target,omitempty"`
	ActionType  string `json:"action_type,omitempty"`
}

type urlJSON struct {
	Time       string   `json:"time"`
	URL        string   `json:"url"`
	Domain     string   `json:"domain"`
	Source     string   `json:"source"`
	Tool       string   `json:"tool"`
	Suspicious bool     `json:"suspicious"`
	Reasons    []string `json:"reasons,omitempty"`
}

type statsJSON struct {
	Sessions int `json:"sessions"`
	Actions  int `json:"actions"`
	Findings int `json:"findings"`
}

// sseClient is a connected browser tab.
type sseClient struct {
	ch chan []byte
}

// Server is the web dashboard HTTP server.
type Server struct {
	source   watcher.EventSource
	sessions int
	port     int

	mu      sync.Mutex
	clients map[*sseClient]bool

	// buffered history for late-joining clients
	actionHistory  []actionJSON
	findingHistory []findingJSON
	urlHistory     []urlJSON
	urlSeen        map[string]bool // dedup by URL
	maxHistory     int
}

// NewServer creates a new web server backed by an EventSource.
func NewServer(source watcher.EventSource, sessions int, port int) *Server {
	return &Server{
		source:     source,
		sessions:   sessions,
		port:       port,
		clients:    make(map[*sseClient]bool),
		urlSeen:    make(map[string]bool),
		maxHistory: 500,
	}
}

// Start starts the HTTP server and the event relay. Blocks.
func (s *Server) Start() error {
	// Relay events from EventSource to SSE clients
	go s.relayActions()
	go s.relayFindings()

	mux := http.NewServeMux()

	// Serve embedded static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// SSE endpoint
	mux.HandleFunc("/api/events", s.handleSSE)

	// REST: get current stats
	mux.HandleFunc("/api/stats", s.handleStats)

	// REST: get buffered history (for late-joining clients)
	mux.HandleFunc("/api/history", s.handleHistory)

	// REST: get URL access history
	mux.HandleFunc("/api/urls", s.handleURLs)

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("AIGaze Web Dashboard → http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statsJSON{
		Sessions: s.sessions,
		Actions:  s.source.ActionCount(),
		Findings: s.source.FindingCount(),
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	actions := make([]actionJSON, len(s.actionHistory))
	findings := make([]findingJSON, len(s.findingHistory))
	urls := make([]urlJSON, len(s.urlHistory))
	copy(actions, s.actionHistory)
	copy(findings, s.findingHistory)
	copy(urls, s.urlHistory)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"actions":  actions,
		"findings": findings,
		"urls":     urls,
	})
}

func (s *Server) handleURLs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	urls := make([]urlJSON, len(s.urlHistory))
	copy(urls, s.urlHistory)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(urls)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := &sseClient{ch: make(chan []byte, 64)}

	s.mu.Lock()
	s.clients[client] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, client)
		s.mu.Unlock()
	}()

	// Keep-alive ticker
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-client.ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) broadcast(eventType string, payload interface{}) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    eventType,
		"payload": payload,
	})
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for client := range s.clients {
		select {
		case client.ch <- data:
		default:
			// slow client, drop message
		}
	}
}

func (s *Server) relayActions() {
	for ev := range s.source.Actions() {
		target := ev.Action.Target
		if len(target) > 80 {
			target = target[:80]
		}
		aj := actionJSON{
			Time:       time.Now().Format("2006-01-02 15:04:05"),
			SessionID:  truncateSessionID(ev.SessionID),
			Tool:       ev.Action.Tool,
			Target:     target,
			ActionType: ev.Action.ActionType,
			Detail:     buildActionDetail(ev.Action.Tool, ev.Action.Arguments, ev.Action.Content),
		}

		s.mu.Lock()
		s.actionHistory = append(s.actionHistory, aj)
		if len(s.actionHistory) > s.maxHistory {
			s.actionHistory = s.actionHistory[len(s.actionHistory)-s.maxHistory:]
		}
		s.mu.Unlock()

		s.broadcast("action", aj)

		// Extract URLs from this action
		now := aj.Time
		s.extractAndBroadcastURLs(ev.Action.Tool, ev.Action.Arguments, ev.Action.Target, ev.Action.Content, now)
	}
}

func (s *Server) relayFindings() {
	for ev := range s.source.Findings() {
		evidence := ev.Finding.Evidence
		if len(evidence) > 80 {
			evidence = evidence[:80]
		}
		fj := findingJSON{
			Time:        time.Now().Format("2006-01-02 15:04:05"),
			Severity:    ev.Finding.Severity,
			RuleID:      ev.Finding.RuleID,
			RuleName:    ev.Finding.RuleName,
			Evidence:    evidence,
			Description: ev.Finding.Description,
			MITRE:       ev.Finding.MITRETechnique + " " + ev.Finding.MITREName,
			Tool:        ev.Finding.Tool,
			Target:      ev.Finding.Target,
			ActionType:  ev.Finding.ActionType,
		}

		s.mu.Lock()
		s.findingHistory = append(s.findingHistory, fj)
		if len(s.findingHistory) > s.maxHistory {
			s.findingHistory = s.findingHistory[len(s.findingHistory)-s.maxHistory:]
		}
		s.mu.Unlock()

		s.broadcast("finding", fj)
	}
}

// buildActionDetail extracts relevant fields from arguments for the detail panel.
func buildActionDetail(tool string, args map[string]interface{}, content string) map[string]interface{} {
	d := make(map[string]interface{})

	switch tool {
	case "read_file":
		if fp, ok := args["filePath"].(string); ok {
			d["filePath"] = fp
		}
		if sl, ok := args["startLine"]; ok {
			d["startLine"] = sl
		}
		if el, ok := args["endLine"]; ok {
			d["endLine"] = el
		}
	case "create_file":
		if fp, ok := args["filePath"].(string); ok {
			d["filePath"] = fp
		}
		if c, ok := args["content"].(string); ok {
			if len(c) > 500 {
				c = c[:500] + "..."
			}
			d["content"] = c
		}
	case "replace_string_in_file", "multi_replace_string_in_file":
		if fp, ok := args["filePath"].(string); ok {
			d["filePath"] = fp
		}
		if os, ok := args["oldString"].(string); ok {
			if len(os) > 300 {
				os = os[:300] + "..."
			}
			d["oldString"] = os
		}
		if ns, ok := args["newString"].(string); ok {
			if len(ns) > 300 {
				ns = ns[:300] + "..."
			}
			d["newString"] = ns
		}
	case "run_in_terminal", "send_to_terminal":
		if cmd, ok := args["command"].(string); ok {
			d["command"] = cmd
		}
		if exp, ok := args["explanation"].(string); ok {
			d["explanation"] = exp
		}
		if goal, ok := args["goal"].(string); ok {
			d["goal"] = goal
		}
	case "fetch_webpage":
		if urls, ok := args["urls"].([]interface{}); ok {
			d["urls"] = urls
		}
		if q, ok := args["query"].(string); ok {
			d["query"] = q
		}
	case "grep_search", "semantic_search":
		if q, ok := args["query"].(string); ok {
			d["query"] = q
		}
		if ip, ok := args["includePattern"].(string); ok {
			d["includePattern"] = ip
		}
	case "list_dir":
		if p, ok := args["path"].(string); ok {
			d["path"] = p
		}
	default:
		// For unknown tools, include raw content (truncated)
		if content != "" {
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			d["raw"] = content
		}
	}

	if len(d) == 0 {
		return nil
	}
	return d
}

func truncateSessionID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// --- URL extraction and classification ---

var webURLRe = regexp.MustCompile(`https?://[^\s"'<>(){}[\]|\\` + "`" + `]+`)

var suspiciousTLDs = map[string]bool{
	".tk": true, ".ml": true, ".ga": true, ".cf": true, ".gq": true,
	".top": true, ".buzz": true, ".work": true, ".click": true, ".loan": true,
	".racing": true, ".download": true, ".win": true, ".bid": true,
	".icu": true,
}

var exfilDomains = map[string]bool{
	"pastebin.com": true, "paste.ee": true, "hastebin.com": true,
	"dpaste.com": true, "ghostbin.com": true,
	"requestbin.com": true, "webhook.site": true, "hookbin.com": true,
	"pipedream.com": true, "beeceptor.com": true,
	"ngrok.io": true, "ngrok-free.app": true, "ngrok.app": true,
	"burpcollaborator.net": true, "interact.sh": true, "oast.fun": true,
	"transfer.sh": true, "file.io": true,
	"api.telegram.org": true,
}

var safeDomainSet = map[string]bool{
	"github.com": true, "raw.githubusercontent.com": true,
	"api.github.com":    true,
	"stackoverflow.com": true, "docs.microsoft.com": true,
	"learn.microsoft.com": true,
	"google.com":          true, "www.google.com": true,
	"pkg.go.dev": true, "pypi.org": true, "npmjs.com": true,
	"en.wikipedia.org": true,
}

func classifyURLWeb(rawURL string, domain string) (bool, []string) {
	if domain == "" {
		return true, []string{"unparseable URL"}
	}
	if safeDomainSet[domain] {
		return false, nil
	}
	var reasons []string
	// IP-based
	if strings.Count(domain, ".") >= 1 {
		allDigits := true
		for _, c := range strings.ReplaceAll(domain, ".", "") {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			reasons = append(reasons, "IP-based URL")
		}
	}
	for tld := range suspiciousTLDs {
		if strings.HasSuffix(domain, tld) {
			reasons = append(reasons, "suspicious TLD: "+tld)
			break
		}
	}
	checkD := domain
	for {
		if exfilDomains[checkD] {
			reasons = append(reasons, "known exfiltration/paste service")
			break
		}
		dot := strings.IndexByte(checkD, '.')
		if dot < 0 || !strings.Contains(checkD[dot+1:], ".") {
			break
		}
		checkD = checkD[dot+1:]
	}
	if strings.HasPrefix(rawURL, "http://") && domain != "localhost" && domain != "127.0.0.1" {
		reasons = append(reasons, "non-HTTPS")
	}
	return len(reasons) > 0, reasons
}

func (s *Server) extractAndBroadcastURLs(tool string, args map[string]interface{}, target string, content string, ts string) {
	var rawURLs []string
	var source string

	switch tool {
	case "fetch_webpage":
		source = "fetch_webpage"
		if urlList, ok := args["urls"].([]interface{}); ok {
			for _, u := range urlList {
				if str, ok := u.(string); ok {
					rawURLs = append(rawURLs, str)
				}
			}
		}
		if target != "" && strings.HasPrefix(target, "http") {
			rawURLs = append(rawURLs, target)
		}
	case "run_in_terminal", "send_to_terminal":
		source = "terminal_command"
		text := target + " " + content
		if cmd, ok := args["command"].(string); ok {
			text += " " + cmd
		}
		rawURLs = webURLRe.FindAllString(text, -1)
	default:
		return
	}

	for _, rawURL := range rawURLs {
		rawURL = strings.TrimRight(rawURL, ".,;:)")
		s.mu.Lock()
		if s.urlSeen[rawURL] {
			s.mu.Unlock()
			continue
		}
		s.urlSeen[rawURL] = true
		s.mu.Unlock()

		u, _ := url.Parse(rawURL)
		domain := ""
		if u != nil {
			domain = strings.ToLower(u.Hostname())
		}

		suspicious, reasons := classifyURLWeb(rawURL, domain)

		uj := urlJSON{
			Time:       ts,
			URL:        rawURL,
			Domain:     domain,
			Source:     source,
			Tool:       tool,
			Suspicious: suspicious,
			Reasons:    reasons,
		}

		s.mu.Lock()
		s.urlHistory = append(s.urlHistory, uj)
		s.mu.Unlock()

		s.broadcast("url", uj)
	}
}

// broadcastStats sends updated stats to all clients.
func (s *Server) broadcastStats() {
	s.broadcast("stats", statsJSON{
		Sessions: s.sessions,
		Actions:  s.source.ActionCount(),
		Findings: s.source.FindingCount(),
	})
}
