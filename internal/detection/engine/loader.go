package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// ParseRuleYAML parses a Rule from raw YAML bytes.
// This is the shared parse core used by both file loaders and the DB loader.
func ParseRuleYAML(data []byte) (Rule, error) {
	var r Rule
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Rule{}, fmt.Errorf("parse rule yaml: %w", err)
	}
	if err := r.Validate(); err != nil {
		return Rule{}, fmt.Errorf("invalid rule: %w", err)
	}
	return r, nil
}

// LoadRuleFromFile reads a single Rule from a YAML file.
func LoadRuleFromFile(path string) (Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Rule{}, fmt.Errorf("load rule %q: %w", path, err)
	}
	r, err := ParseRuleYAML(data)
	if err != nil {
		return Rule{}, fmt.Errorf("load rule %q: %w", path, err)
	}
	return r, nil
}

// LoadRulesFromDirectory loads all YAML files from a directory.
func LoadRulesFromDirectory(dir string) ([]Rule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read rules directory %q: %w", dir, err)
	}

	rules := make([]Rule, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		r, err := LoadRuleFromFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		rules = append(rules, r)
	}
	return rules, nil
}
