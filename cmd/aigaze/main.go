package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/aigaze-sec/aigaze/internal/discover"
	"github.com/aigaze-sec/aigaze/internal/engine"
	"github.com/aigaze-sec/aigaze/internal/parser"
	"github.com/aigaze-sec/aigaze/internal/tui"
	"github.com/aigaze-sec/aigaze/internal/watcher"
	"github.com/aigaze-sec/aigaze/internal/web"
)

var version = "0.3.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "aigaze",
		Short: "AIGaze — Audit AI agent actions from transcript files",
	}

	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(discoverCmd())
	rootCmd.AddCommand(scanCmd())
	rootCmd.AddCommand(watchCmd())
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(ruleCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("aigaze %s (go)\n", version)
		},
	}
}

func discoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Auto-discover transcript files on this machine",
		Run: func(cmd *cobra.Command, args []string) {
			locations := discover.Discover()
			if len(locations) == 0 {
				fmt.Println("No transcript files found.")
				fmt.Println("\nChecked known paths for:")
				fmt.Println("  - VS Code Copilot Chat")
				fmt.Println("  - Cursor")
				fmt.Println("  - Claude Code")
				fmt.Println("  - Windsurf")
				return
			}

			fmt.Printf("Found transcripts in %d location(s):\n\n", len(locations))
			total := 0
			for _, loc := range locations {
				sizeStr := humanSize(loc.TotalSize)
				fmt.Printf("  [%s] %s\n", loc.Source, loc.Path)
				fmt.Printf("  %d sessions, %s\n\n", loc.SessionCount, sizeStr)
				total += loc.SessionCount
			}
			fmt.Printf("Total: %d sessions\n", total)
			fmt.Println("\nRun 'aigaze scan --auto' to scan all discovered transcripts.")
		},
	}
}

func scanCmd() *cobra.Command {
	var auto bool
	var outputFormat string
	var workspace []string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan transcript files for security findings",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var files []string

			if auto {
				locations := discover.Discover()
				for _, loc := range locations {
					jsonlFiles := findJSONLFiles(loc.Path)
					files = append(files, jsonlFiles...)
				}
			} else if len(args) > 0 {
				files = findJSONLFiles(args[0])
			} else {
				fmt.Fprintln(os.Stderr, "Error: Provide a PATH or use --auto to discover transcripts.")
				os.Exit(1)
			}

			if len(files) == 0 {
				fmt.Fprintln(os.Stderr, "No .jsonl transcript files found.")
				os.Exit(1)
			}

			for _, f := range files {
				session, err := parser.ParseTranscript(f)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", f, err)
					continue
				}

				result := engine.ScanSession(session, workspace)

				if outputFormat == "json" {
					data, _ := json.MarshalIndent(result, "", "  ")
					if outputFile != "" {
						os.WriteFile(outputFile, data, 0644)
						fmt.Printf("Report written to %s\n", outputFile)
					} else {
						fmt.Println(string(data))
					}
				} else {
					printTerminalReport(session, result)
				}
			}
		},
	}

	cmd.Flags().BoolVar(&auto, "auto", false, "Auto-discover and scan all transcripts")
	cmd.Flags().StringVar(&outputFormat, "format", "terminal", "Output format: terminal or json")
	cmd.Flags().StringSliceVar(&workspace, "workspace", nil, "Workspace root path(s)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write output to file")

	return cmd
}

func watchCmd() *cobra.Command {
	var workspace []string
	var replayFile string
	var speed int

	cmd := &cobra.Command{
		Use:   "watch [path]",
		Short: "Real-time monitoring of AI agent actions",
		Long: `Watches transcript files for new activity and displays a live TUI dashboard.
Without PATH, auto-discovers all known transcript locations.

Use --replay to open a JSONL file in offline mode for post-hoc analysis.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Offline replay mode
			if replayFile != "" {
				session, err := parser.ParseTranscript(replayFile)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", replayFile, err)
					os.Exit(1)
				}

				re := watcher.NewReplay(session, workspace, speed)
				go re.Start()

				model := tui.NewModel(re, 1, true)
				p := tea.NewProgram(model, tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				return
			}

			// Real-time mode
			we, err := watcher.New(workspace)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating watcher: %v\n", err)
				os.Exit(1)
			}

			var sessionCount int
			if len(args) > 0 {
				if err := we.AddDirectory(args[0]); err != nil {
					fmt.Fprintf(os.Stderr, "Error watching %s: %v\n", args[0], err)
					os.Exit(1)
				}
				sessionCount = len(findJSONLFiles(args[0]))
			} else {
				locations := we.AutoDiscover()
				for _, loc := range locations {
					sessionCount += loc.SessionCount
				}
			}

			// Start watcher in background
			go we.Start()

			// Run TUI
			model := tui.NewModel(we, sessionCount, false)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringSliceVar(&workspace, "workspace", nil, "Workspace root path(s)")
	cmd.Flags().StringVar(&replayFile, "replay", "", "Replay a JSONL transcript file offline in the TUI")
	cmd.Flags().IntVar(&speed, "speed", 0, "Replay delay in ms between events (0 = instant)")
	return cmd
}

func serveCmd() *cobra.Command {
	var workspace []string
	var replayFiles []string
	var replayDir string
	var speed int
	var port int

	cmd := &cobra.Command{
		Use:   "serve [path]",
		Short: "Start web dashboard for AI agent action monitoring",
		Long: `Starts an HTTP server with a real-time web dashboard.
Without PATH, auto-discovers all known transcript locations.

Use --replay to serve a JSONL transcript file for offline analysis.
Use multiple --replay flags or --replay-dir to load multiple sessions.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var source watcher.EventSource
			var sessionCount int

			// Collect all replay files
			allFiles := append([]string{}, replayFiles...)
			if replayDir != "" {
				allFiles = append(allFiles, findJSONLFiles(replayDir)...)
			}

			if len(allFiles) > 0 {
				// Multi-session replay mode
				var allSessions []*parser.Session
				for _, f := range allFiles {
					sessions, err := parser.ParseMultiSession(f)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", f, err)
						continue
					}
					allSessions = append(allSessions, sessions...)
				}
				if len(allSessions) == 0 {
					fmt.Fprintln(os.Stderr, "No sessions found in replay files.")
					os.Exit(1)
				}
				sessionCount = len(allSessions)
				multi := watcher.NewMultiReplay(allSessions, workspace, speed)
				go multi.Start()
				source = multi
			} else {
				// Real-time mode
				we, err := watcher.New(workspace)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating watcher: %v\n", err)
					os.Exit(1)
				}

				if len(args) > 0 {
					if err := we.AddDirectory(args[0]); err != nil {
						fmt.Fprintf(os.Stderr, "Error watching %s: %v\n", args[0], err)
						os.Exit(1)
					}
					sessionCount = len(findJSONLFiles(args[0]))
				} else {
					locations := we.AutoDiscover()
					for _, loc := range locations {
						sessionCount += loc.SessionCount
					}
				}
				go we.Start()
				source = we
			}

			srv := web.NewServer(source, sessionCount, port)
			if err := srv.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "HTTP port")
	cmd.Flags().StringSliceVar(&workspace, "workspace", nil, "Workspace root path(s)")
	cmd.Flags().StringArrayVar(&replayFiles, "replay", nil, "Replay JSONL transcript file(s) offline (repeatable)")
	cmd.Flags().StringVar(&replayDir, "replay-dir", "", "Replay all JSONL files in a directory")
	cmd.Flags().IntVar(&speed, "speed", 0, "Replay delay in ms between events (0 = instant)")
	return cmd
}

// --- Rule commands ---

func ruleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage and test detection rules",
	}
	cmd.AddCommand(ruleTestCmd())
	cmd.AddCommand(ruleListCmd())
	return cmd
}

func ruleTestCmd() *cobra.Command {
	var shouldMatch string
	var shouldNotMatch string

	cmd := &cobra.Command{
		Use:   "test <rule.yaml>",
		Short: "Test a YAML rule against fixture transcripts",
		Long: `Validate a detection rule by running it against should-match and
should-not-match fixture transcripts.

Example:
  aigaze rule test rules/R100.yaml \
    --should-match fixtures/R100-trigger.jsonl \
    --should-not-match fixtures/R100-safe.jsonl`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			rulePath := args[0]
			data, err := os.ReadFile(rulePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading rule file: %v\n", err)
				os.Exit(1)
			}

			rule, err := engine.ParseRuleBytes(data, filepath.Base(rulePath))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing rule: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Testing rule %s (%s)\n", rule.ID, rule.Name)
			fmt.Printf("  Check: %s  Severity: %s\n\n", rule.CheckType, rule.Severity)

			passed := true

			if shouldMatch != "" {
				ok := runRuleTest(rule, shouldMatch, true)
				if !ok {
					passed = false
				}
			}

			if shouldNotMatch != "" {
				ok := runRuleTest(rule, shouldNotMatch, false)
				if !ok {
					passed = false
				}
			}

			if shouldMatch == "" && shouldNotMatch == "" {
				fmt.Fprintln(os.Stderr, "Provide --should-match and/or --should-not-match fixtures.")
				os.Exit(1)
			}

			fmt.Println()
			if passed {
				fmt.Println("✅ All assertions passed.")
			} else {
				fmt.Println("❌ Some assertions failed.")
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&shouldMatch, "should-match", "", "JSONL fixture that should trigger the rule")
	cmd.Flags().StringVar(&shouldNotMatch, "should-not-match", "", "JSONL fixture that should NOT trigger the rule")
	return cmd
}

func ruleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all loaded detection rules",
		Run: func(cmd *cobra.Command, args []string) {
			rules, err := engine.LoadAllRules()
			if err != nil {
				rules = engine.DefaultRules()
			}
			fmt.Printf("%-6s %-10s %-35s %-15s %s\n", "ID", "Severity", "Name", "Check", "MITRE")
			fmt.Printf("%-6s %-10s %-35s %-15s %s\n", "------", "----------", "-----------------------------------", "---------------", "----------")
			for _, r := range rules {
				fmt.Printf("%-6s %-10s %-35s %-15s %s\n", r.ID, r.Severity, r.Name, r.CheckType, r.MITRETechnique)
			}
		},
	}
}

func runRuleTest(rule engine.Rule, fixturePath string, expectMatch bool) bool {
	session, err := parser.ParseTranscript(fixturePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error parsing %s: %v\n", fixturePath, err)
		return false
	}

	result := engine.ScanSessionWithRules(session, nil, []engine.Rule{rule})
	activeFindings := result.FindingCount()

	label := "should-match"
	if !expectMatch {
		label = "should-not-match"
	}

	if expectMatch {
		if activeFindings > 0 {
			fmt.Printf("  PASS  %s → %d finding(s) detected (%s)\n", label, activeFindings, filepath.Base(fixturePath))
			return true
		}
		fmt.Printf("  FAIL  %s → 0 findings, expected ≥1 (%s)\n", label, filepath.Base(fixturePath))
		return false
	}

	// expect no match
	if activeFindings == 0 {
		fmt.Printf("  PASS  %s → 0 findings (%s)\n", label, filepath.Base(fixturePath))
		return true
	}
	fmt.Printf("  FAIL  %s → %d finding(s), expected 0 (%s)\n", label, activeFindings, filepath.Base(fixturePath))
	for _, f := range result.Findings {
		if !f.Suppressed {
			fmt.Printf("        → %s: %s\n", f.Tool, f.Evidence)
		}
	}
	return false
}

// --- Helpers ---

func findJSONLFiles(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if !info.IsDir() {
		if filepath.Ext(path) == ".jsonl" {
			return []string{path}
		}
		return nil
	}

	var files []string
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(p) == ".jsonl" {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func humanSize(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	size := float64(bytes)
	for _, unit := range units {
		if size < 1024 {
			return fmt.Sprintf("%.1f%s", size, unit)
		}
		size /= 1024
	}
	return fmt.Sprintf("%.1fTB", size)
}

func printTerminalReport(session *parser.Session, result *engine.ScanResult) {
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Printf("  Session: %s\n", session.SessionID)
	fmt.Printf("  Model:   %s\n", session.Model)
	fmt.Printf("  Actions: %d  |  Turns: %d\n", len(session.Actions), len(session.Turns))
	fmt.Println("══════════════════════════════════════════════════════════════")

	if result.FindingCount() == 0 {
		fmt.Println("\n  ✅ No security findings.")
	} else {
		fmt.Printf("\n  ⚠ %d finding(s):\n\n", result.FindingCount())
		fmt.Printf("  %-6s %-10s %-30s %-15s %s\n", "Rule", "Severity", "Name", "MITRE", "Evidence")
		fmt.Printf("  %-6s %-10s %-30s %-15s %s\n", "------", "----------", "------------------------------", "---------------", "--------")

		for _, f := range result.Findings {
			if f.Suppressed {
				continue
			}
			evidence := f.Evidence
			if len(evidence) > 40 {
				evidence = evidence[:40] + "..."
			}
			mitre := f.MITRETechnique
			if len(mitre) > 15 {
				mitre = mitre[:15]
			}
			fmt.Printf("  %-6s %-10s %-30s %-15s %s\n", f.RuleID, f.Severity, f.RuleName, mitre, evidence)
		}
	}

	if result.SuppressedCount() > 0 {
		fmt.Printf("\n  🔇 %d suppressed (false positive)\n", result.SuppressedCount())
	}
	fmt.Println()
}
