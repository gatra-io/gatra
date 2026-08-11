package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gatra-io/gatra/internal/errors"
)

type AccumulatorType string

const (
	AccumulatorSum      AccumulatorType = "sum"
	AccumulatorCount    AccumulatorType = "count"
	AccumulatorUniqueSet AccumulatorType = "unique_set"
)

type Limits struct {
	MaxPerCall    *float64 `json:"max_per_call,omitempty"`
	MaxCumulative *float64 `json:"max_cumulative,omitempty"`
}

type Accumulator struct {
	Type       AccumulatorType `json:"type"`
	UniquePath string          `json:"unique_path,omitempty"`
}

type TimeWindow struct {
	Enabled       bool   `json:"enabled"`
	ResetSchedule string `json:"reset_schedule,omitempty"`
	Timezone      string `json:"timezone,omitempty"`
}

type Rule struct {
	RuleID      string      `json:"rule_id"`
	ToolPattern string      `json:"tool_pattern"`
	ValuePath   string      `json:"value_path"`
	Limits      Limits      `json:"limits"`
	Accumulator Accumulator `json:"accumulator"`
	TimeWindow  *TimeWindow `json:"time_window,omitempty"`
	Condition   string      `json:"condition,omitempty"`
}

type Policy struct {
	Version string `json:"version"`
	Rules   []Rule `json:"rules"`
}

// LoadPolicy reads, parses, and validates a JSON policy file.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read policy file '%s': %v", errors.ErrInvalidConfiguration, path, err)
	}

	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("%w: failed to parse policy JSON: %v", errors.ErrInvalidConfiguration, err)
	}

	if err := policy.Validate(); err != nil {
		return nil, err
	}

	return &policy, nil
}

// Validate asserts the structural and logical invariants of loaded rules.
func (p *Policy) Validate() error {
	if p.Version == "" {
		return fmt.Errorf("%w: missing policy version", errors.ErrInvalidConfiguration)
	}

	if len(p.Rules) == 0 {
		return fmt.Errorf("%w: policy must contain at least one rule", errors.ErrInvalidConfiguration)
	}

	ruleIDs := make(map[string]bool)

	for i, rule := range p.Rules {
		if rule.RuleID == "" {
			return fmt.Errorf("%w: rule index %d missing rule_id", errors.ErrInvalidConfiguration, i)
		}

		if ruleIDs[rule.RuleID] {
			return fmt.Errorf("%w: duplicate rule_id '%s'", errors.ErrInvalidConfiguration, rule.RuleID)
		}
		ruleIDs[rule.RuleID] = true

		if rule.ToolPattern == "" {
			return fmt.Errorf("%w: rule '%s' missing tool_pattern", errors.ErrInvalidConfiguration, rule.RuleID)
		}

		switch rule.Accumulator.Type {
		case AccumulatorSum, AccumulatorCount, AccumulatorUniqueSet:
			// Valid accumulator type
		default:
			return fmt.Errorf("%w: rule '%s' has invalid accumulator type '%s'", errors.ErrInvalidConfiguration, rule.RuleID, rule.Accumulator.Type)
		}

		if rule.Limits.MaxPerCall != nil && rule.Limits.MaxCumulative != nil {
			if *rule.Limits.MaxPerCall > *rule.Limits.MaxCumulative {
				return fmt.Errorf("%w: rule '%s' max_per_call (%f) exceeds max_cumulative (%f)",
					errors.ErrInvalidConfiguration, rule.RuleID, *rule.Limits.MaxPerCall, *rule.Limits.MaxCumulative)
			}
		}
	}

	return nil
}