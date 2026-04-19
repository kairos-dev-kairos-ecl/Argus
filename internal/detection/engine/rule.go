package engine

import "fmt"

// Rule is a detection rule loaded from YAML.
type Rule struct {
	ID             string     `yaml:"id" json:"id"`
	Name           string     `yaml:"name" json:"name"`
	Description    string     `yaml:"description" json:"description"`
	Tier           int        `yaml:"tier" json:"tier"`
	Enabled        bool       `yaml:"enabled" json:"enabled"`
	Severity       int        `yaml:"severity" json:"severity"`
	Conditions     Conditions `yaml:"conditions" json:"conditions"`
	Action         Action     `yaml:"action" json:"action"`
	RequiresKairos bool       `yaml:"requires_kairos" json:"requires_kairos"`
}

// Conditions defines what a signal must look like to trigger the rule.
type Conditions struct {
	Layer                string        `yaml:"layer" json:"layer"`
	CategoryPrefix       string        `yaml:"category_prefix" json:"category_prefix"`
	SeverityGte          int           `yaml:"severity_gte" json:"severity_gte"`
	BaselineDeviationGte float32       `yaml:"baseline_deviation_gte" json:"baseline_deviation_gte"`
	Temporal             *TemporalCond `yaml:"temporal" json:"temporal"`
}

// TemporalCond defines Tier 3 frequency-based triggering.
type TemporalCond struct {
	CountGte      int `yaml:"count_gte" json:"count_gte"`
	WindowSeconds int `yaml:"window_seconds" json:"window_seconds"`
}

// Action describes what alert to emit when the rule fires.
type Action struct {
	Title          string `yaml:"title" json:"title"`
	Description    string `yaml:"description" json:"description"`
	MitreTactic    string `yaml:"mitre_tactic" json:"mitre_tactic"`
	MitreTechnique string `yaml:"mitre_technique" json:"mitre_technique"`
}

// Validate returns an error if the rule is structurally invalid.
func (r Rule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("rule id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("rule %q: name is required", r.ID)
	}
	if r.Tier < 1 || r.Tier > 3 {
		return fmt.Errorf("rule %q: tier must be 1, 2, or 3 (got %d)", r.ID, r.Tier)
	}
	if r.Severity < 1 || r.Severity > 5 {
		return fmt.Errorf("rule %q: severity must be 1-5 (got %d)", r.ID, r.Severity)
	}
	if r.Action.Title == "" {
		return fmt.Errorf("rule %q: action.title is required", r.ID)
	}
	if r.Tier == 3 {
		if r.Conditions.Temporal == nil {
			return fmt.Errorf("rule %q: tier 3 rules require conditions.temporal", r.ID)
		}
		if r.Conditions.Temporal.CountGte < 1 || r.Conditions.Temporal.WindowSeconds < 1 {
			return fmt.Errorf("rule %q: temporal.count_gte and temporal.window_seconds must be >= 1", r.ID)
		}
	}
	return nil
}
