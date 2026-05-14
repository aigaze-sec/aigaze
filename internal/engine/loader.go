package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aigaze-sec/aigaze/rules"
	"gopkg.in/yaml.v3"
)

// RuleYAML is the YAML schema for a detection rule.
type RuleYAML struct {
	ID                   string   `yaml:"id"`
	Name                 string   `yaml:"name"`
	Description          string   `yaml:"description"`
	Severity             string   `yaml:"severity"`
	MITRETechnique       string   `yaml:"mitre_technique"`
	MITREName            string   `yaml:"mitre_name"`
	ActionTypes          []string `yaml:"action_types"`
	Check                string   `yaml:"check"`
	Patterns             []string `yaml:"patterns"`
	SuppressPlaceholders bool     `yaml:"suppress_placeholders"`
}

// parseRuleYAML converts a RuleYAML into the engine Rule struct.
func parseRuleYAML(ry RuleYAML) (Rule, error) {
	r := Rule{
		ID:                   ry.ID,
		Name:                 ry.Name,
		Description:          ry.Description,
		Severity:             ry.Severity,
		MITRETechnique:       ry.MITRETechnique,
		MITREName:            ry.MITREName,
		ActionTypes:          ry.ActionTypes,
		CheckType:            ry.Check,
		SuppressPlaceholders: ry.SuppressPlaceholders,
	}

	for _, p := range ry.Patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return r, fmt.Errorf("rule %s: invalid pattern %q: %w", ry.ID, p, err)
		}
		r.Patterns = append(r.Patterns, re)
	}

	return r, nil
}

// LoadBuiltinRules loads the embedded YAML rules from rules/builtin/.
func LoadBuiltinRules() ([]Rule, error) {
	return loadRulesFromFS(rules.BuiltinFS, "builtin")
}

// LoadExternalRules loads YAML rules from a filesystem directory.
// Returns empty slice (not error) if the directory doesn't exist.
func LoadExternalRules(dir string) ([]Rule, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	var rules []Rule
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		r, err := ParseRuleBytes(data, entry.Name())
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	return rules, nil
}

// LoadAllRules loads builtin + external rules. External rules override builtin by ID.
func LoadAllRules(externalDirs ...string) ([]Rule, error) {
	builtin, err := LoadBuiltinRules()
	if err != nil {
		return nil, fmt.Errorf("loading builtin rules: %w", err)
	}

	ruleMap := make(map[string]Rule)
	for _, r := range builtin {
		ruleMap[r.ID] = r
	}

	for _, dir := range externalDirs {
		ext, err := LoadExternalRules(dir)
		if err != nil {
			return nil, fmt.Errorf("loading rules from %s: %w", dir, err)
		}
		for _, r := range ext {
			ruleMap[r.ID] = r // override by ID
		}
	}

	// Collect in sorted order (R1, R2, ..., R10, R11, ...)
	var result []Rule
	for _, r := range ruleMap {
		result = append(result, r)
	}
	// Sort by ID
	sortRules(result)
	return result, nil
}

func loadRulesFromFS(fsys fs.FS, root string) ([]Rule, error) {
	var rules []Rule
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isYAMLFile(d.Name()) {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		r, err := ParseRuleBytes(data, d.Name())
		if err != nil {
			return err
		}
		rules = append(rules, r)
		return nil
	})
	return rules, err
}

// ParseRuleBytes parses a single YAML rule from raw bytes.
func ParseRuleBytes(data []byte, filename string) (Rule, error) {
	var ry RuleYAML
	if err := yaml.Unmarshal(data, &ry); err != nil {
		return Rule{}, fmt.Errorf("parsing %s: %w", filename, err)
	}
	if ry.ID == "" {
		return Rule{}, fmt.Errorf("%s: missing rule id", filename)
	}
	if ry.Check == "" {
		return Rule{}, fmt.Errorf("%s (rule %s): missing check type", filename, ry.ID)
	}
	return parseRuleYAML(ry)
}

func isYAMLFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

func sortRules(rules []Rule) {
	// Simple insertion sort — rule count is small
	for i := 1; i < len(rules); i++ {
		for j := i; j > 0 && ruleIDLess(rules[j].ID, rules[j-1].ID); j-- {
			rules[j], rules[j-1] = rules[j-1], rules[j]
		}
	}
}

// ruleIDLess compares rule IDs like R1 < R2 < R10.
func ruleIDLess(a, b string) bool {
	na := extractNum(a)
	nb := extractNum(b)
	if na != nb {
		return na < nb
	}
	return a < b
}

func extractNum(id string) int {
	n := 0
	for _, c := range id {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
