package engine

import (
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/aigaze-sec/aigaze/internal/parser"
)

// URLRecord represents a single URL access observed during a session.
type URLRecord struct {
	URL        string   `json:"url"`
	Domain     string   `json:"domain"`
	Source     string   `json:"source"` // "fetch_webpage", "curl", "wget", etc.
	Tool       string   `json:"tool"`   // original tool name
	Suspicious bool     `json:"suspicious"`
	Reasons    []string `json:"reasons,omitempty"`
	Timestamp  string   `json:"timestamp"`
	TurnID     string   `json:"turn_id"`
}

var urlRe = regexp.MustCompile(`https?://[^\s"'<>(){}[\]|\\` + "`" + `]+`)

// Suspicious TLDs frequently used in phishing / data exfil
var suspiciousTLDs = map[string]bool{
	".tk": true, ".ml": true, ".ga": true, ".cf": true, ".gq": true,
	".top": true, ".buzz": true, ".work": true, ".click": true, ".loan": true,
	".racing": true, ".download": true, ".win": true, ".bid": true,
	".cam": true, ".rest": true, ".icu": true,
}

// Known paste / data exfil / webhook domains
var exfilDomains = map[string]bool{
	"pastebin.com": true, "paste.ee": true, "hastebin.com": true,
	"dpaste.com": true, "ghostbin.com": true,
	"requestbin.com": true, "webhook.site": true, "hookbin.com": true,
	"pipedream.com": true, "beeceptor.com": true,
	"ngrok.io": true, "ngrok-free.app": true, "ngrok.app": true,
	"burpcollaborator.net": true, "interact.sh": true, "oast.fun": true,
	"transfer.sh": true, "file.io": true,
	"discord.com": true, "discordapp.com": true,
	"telegram.org": true, "api.telegram.org": true,
}

// Known safe domains (should not flag)
var safeDomains = map[string]bool{
	"github.com": true, "raw.githubusercontent.com": true,
	"api.github.com":    true,
	"stackoverflow.com": true, "docs.microsoft.com": true,
	"learn.microsoft.com": true,
	"google.com":          true, "www.google.com": true,
	"pkg.go.dev": true, "pypi.org": true, "npmjs.com": true,
	"registry.npmjs.org": true,
	"en.wikipedia.org":   true,
}

// ExtractURLs extracts all URL accesses from a session with suspicious classification.
func ExtractURLs(session *parser.Session) []URLRecord {
	var records []URLRecord
	seen := make(map[string]bool) // dedup by URL+TurnID

	for _, action := range session.Actions {
		var urls []string
		var source string

		switch action.ActionType {
		case "network_fetch":
			// fetch_webpage: URLs from arguments
			source = "fetch_webpage"
			if urlList, ok := action.Arguments["urls"].([]interface{}); ok {
				for _, u := range urlList {
					if s, ok := u.(string); ok {
						urls = append(urls, s)
					}
				}
			}
			// Also check target
			if action.Target != "" && strings.HasPrefix(action.Target, "http") {
				urls = append(urls, action.Target)
			}

		case "terminal_exec", "terminal_input":
			// Extract URLs from commands (curl, wget, etc.)
			source = "terminal_command"
			text := action.Target + " " + action.Content
			if cmd, ok := action.Arguments["command"].(string); ok {
				text += " " + cmd
			}
			found := urlRe.FindAllString(text, -1)
			urls = append(urls, found...)
		}

		for _, rawURL := range urls {
			// Clean trailing punctuation
			rawURL = strings.TrimRight(rawURL, ".,;:)")

			key := rawURL + "|" + action.TurnID
			if seen[key] {
				continue
			}
			seen[key] = true

			rec := URLRecord{
				URL:       rawURL,
				Source:    source,
				Tool:      action.Tool,
				Timestamp: action.Timestamp,
				TurnID:    action.TurnID,
			}

			rec.Domain = extractDomain(rawURL)
			rec.Suspicious, rec.Reasons = classifyURL(rawURL, rec.Domain)
			records = append(records, rec)
		}
	}

	return records
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	return strings.ToLower(host)
}

func classifyURL(rawURL string, domain string) (bool, []string) {
	if domain == "" {
		return true, []string{"unparseable URL"}
	}

	// Safe domains pass immediately
	if safeDomains[domain] {
		return false, nil
	}

	var reasons []string

	// Check if IP address instead of domain
	if ip := net.ParseIP(domain); ip != nil {
		reasons = append(reasons, "IP-based URL (no domain name)")
	}

	// Check suspicious TLDs
	for tld := range suspiciousTLDs {
		if strings.HasSuffix(domain, tld) {
			reasons = append(reasons, "suspicious TLD: "+tld)
			break
		}
	}

	// Check exfil / paste / webhook domains
	// Check both exact and parent domain
	checkDomain := domain
	for {
		if exfilDomains[checkDomain] {
			reasons = append(reasons, "known data exfiltration / paste service")
			break
		}
		dot := strings.IndexByte(checkDomain, '.')
		if dot < 0 {
			break
		}
		checkDomain = checkDomain[dot+1:]
		if !strings.Contains(checkDomain, ".") {
			break
		}
	}

	// Check non-HTTPS
	if strings.HasPrefix(rawURL, "http://") && domain != "localhost" && domain != "127.0.0.1" {
		reasons = append(reasons, "non-HTTPS connection")
	}

	// Check for encoded/obfuscated patterns
	if strings.Contains(rawURL, "%2F%2F") || strings.Contains(rawURL, "%3A") {
		reasons = append(reasons, "encoded URL characters (possible obfuscation)")
	}

	// Check for unusually long subdomains (DNS tunneling indicator)
	parts := strings.Split(domain, ".")
	for _, part := range parts {
		if len(part) > 40 {
			reasons = append(reasons, "unusually long subdomain (possible DNS tunneling)")
			break
		}
	}

	return len(reasons) > 0, reasons
}
