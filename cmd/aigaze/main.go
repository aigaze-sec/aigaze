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
)

var version = "0.2.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "aigaze",
		Short: "AIGaze — Audit AI agent actions from transcript files",
	}

	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(discoverCmd())
	rootCmd.AddCommand(scanCmd())
	rootCmd.AddCommand(watchCmd())

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

	cmd := &cobra.Command{
		Use:   "watch [path]",
		Short: "Real-time monitoring of AI agent actions",
		Long:  "Watches transcript files for new activity and displays a live TUI dashboard.\nWithout PATH, auto-discovers all known transcript locations.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
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
			model := tui.NewModel(we, sessionCount)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringSliceVar(&workspace, "workspace", nil, "Workspace root path(s)")
	return cmd
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
