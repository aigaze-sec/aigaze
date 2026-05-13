package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Location represents a discovered transcript directory.
type Location struct {
	Path         string `json:"path"`
	Source       string `json:"source"` // "vscode-copilot", "cursor", "claude-code"
	SessionCount int    `json:"session_count"`
	TotalSize    int64  `json:"total_size_bytes"`
}

// knownPattern defines a source name and glob patterns per OS.
type knownPattern struct {
	Source   string
	Patterns []string
}

func knownPatterns(home string) []knownPattern {
	return []knownPattern{
		{
			Source: "vscode-copilot",
			Patterns: []string{
				// Linux / WSL
				filepath.Join(home, ".vscode-server", "data", "User", "workspaceStorage", "*", "GitHub.copilot-chat", "transcripts"),
				// macOS
				filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage", "*", "GitHub.copilot-chat", "transcripts"),
				// Windows
				filepath.Join(home, "AppData", "Roaming", "Code", "User", "workspaceStorage", "*", "GitHub.copilot-chat", "transcripts"),
				// VS Code Insiders
				filepath.Join(home, ".vscode-server-insiders", "data", "User", "workspaceStorage", "*", "GitHub.copilot-chat", "transcripts"),
				filepath.Join(home, "Library", "Application Support", "Code - Insiders", "User", "workspaceStorage", "*", "GitHub.copilot-chat", "transcripts"),
			},
		},
		{
			Source: "cursor",
			Patterns: []string{
				filepath.Join(home, ".cursor", "transcripts"),
				filepath.Join(home, "Library", "Application Support", "Cursor", "User", "workspaceStorage", "*", "transcripts"),
				filepath.Join(home, "AppData", "Roaming", "Cursor", "User", "workspaceStorage", "*", "transcripts"),
			},
		},
		{
			Source: "claude-code",
			Patterns: []string{
				filepath.Join(home, ".claude", "projects", "*"),
			},
		},
		{
			Source: "windsurf",
			Patterns: []string{
				filepath.Join(home, ".windsurf", "transcripts"),
				filepath.Join(home, "Library", "Application Support", "Windsurf", "User", "workspaceStorage", "*", "transcripts"),
			},
		},
	}
}

// Discover finds all transcript directories on the local system.
func Discover() []Location {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return DiscoverIn(home)
}

// DiscoverIn finds transcript directories starting from a given home directory.
func DiscoverIn(home string) []Location {
	var locations []Location

	for _, kp := range knownPatterns(home) {
		for _, pattern := range kp.Patterns {
			// Skip patterns for other OS
			if runtime.GOOS != "darwin" && strings.Contains(pattern, "Library/Application Support") {
				continue
			}
			if runtime.GOOS != "windows" && strings.Contains(pattern, "AppData") {
				continue
			}

			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}

			for _, match := range matches {
				jsonlFiles := findJSONL(match)
				if len(jsonlFiles) == 0 {
					continue
				}

				var totalSize int64
				for _, f := range jsonlFiles {
					info, err := os.Stat(f)
					if err == nil {
						totalSize += info.Size()
					}
				}

				locations = append(locations, Location{
					Path:         match,
					Source:       kp.Source,
					SessionCount: len(jsonlFiles),
					TotalSize:    totalSize,
				})
			}
		}
	}

	return locations
}

// findJSONL returns all .jsonl files in a directory.
func findJSONL(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

// AllTranscriptDirs returns all unique transcript directory paths.
func AllTranscriptDirs() []string {
	locs := Discover()
	var dirs []string
	for _, loc := range locs {
		dirs = append(dirs, loc.Path)
	}
	return dirs
}
