package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gatra-io/gatra/internal/config"
)

// MCPToolSchema represents the standard MCP tools/list definition JSON Schema
type MCPToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema MCPInputSchema `json:"inputSchema"`
}

type MCPInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]MCPProperty `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type MCPProperty struct {
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
	Minimum     *float64  `json:"minimum,omitempty"`
	Maximum     *float64  `json:"maximum,omitempty"`
	Pattern     string    `json:"pattern,omitempty"`
}

type ToolListContainer struct {
	Tools []MCPToolSchema `json:"tools"`
}

// Options configures the policy discovery engine behavior
type Options struct {
	DefaultMaxPerCall    float64
	DefaultMaxCumulative float64
	ResetSchedule        string
	Timezone             string
	EnableInjectionGuard bool
	EnablePathGuard      bool
	StrictEnums          bool
}

// DefaultOptions provides balanced baseline discovery settings
func DefaultOptions() Options {
	return Options{
		DefaultMaxPerCall:    100.00,
		DefaultMaxCumulative: 1000.00,
		ResetSchedule:        "@daily",
		Timezone:             "UTC",
		EnableInjectionGuard: true,
		EnablePathGuard:      true,
		StrictEnums:          true,
	}
}

// GeneratePolicyFromMCP parses MCP tool schema JSON and constructs a candidate policy
func GeneratePolicyFromMCP(mcpSchemaPath string, opts Options) (*config.Policy, error) {
	data, err := os.ReadFile(mcpSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP schema file '%s': %w", mcpSchemaPath, err)
	}

	var container ToolListContainer
	if err := json.Unmarshal(data, &container); err != nil {
		var tools []MCPToolSchema
		if errArr := json.Unmarshal(data, &tools); errArr != nil {
			return nil, fmt.Errorf("failed to parse MCP schema JSON: %w (array error: %v)", err, errArr)
		}
		container.Tools = tools
	}

	if len(container.Tools) == 0 {
		return nil, fmt.Errorf("no valid MCP tool definitions discovered in '%s'", mcpSchemaPath)
	}

	policy := &config.Policy{
		Version: "v1",
		Rules:   make([]config.Rule, 0),
	}

	for _, tool := range container.Tools {
		rules := analyzeToolSchema(tool, opts)
		policy.Rules = append(policy.Rules, rules...)
	}

	return policy, nil
}

func analyzeToolSchema(tool MCPToolSchema, opts Options) []config.Rule {
	rules := make([]config.Rule, 0)

	for paramName, prop := range tool.InputSchema.Properties {
		paramLower := strings.ToLower(paramName)

		// 1. Numeric Parameter Guardrails (Limits & Accumulators)
		if prop.Type == "number" || prop.Type == "integer" {
			maxPerCall := opts.DefaultMaxPerCall
			if prop.Maximum != nil {
				maxPerCall = *prop.Maximum
			}

			maxCumulative := opts.DefaultMaxCumulative

			rule := config.Rule{
				RuleID:      fmt.Sprintf("%s_%s_numeric_limit", sanitizeID(tool.Name), sanitizeID(paramName)),
				ToolPattern: tool.Name,
				ValuePath:   fmt.Sprintf("$.%s", paramName),
				Limits: config.Limits{
					MaxPerCall:    floatPtr(maxPerCall),
					MaxCumulative: floatPtr(maxCumulative),
				},
				Accumulator: config.Accumulator{
					Type: config.AccumulatorSum,
				},
				TimeWindow: &config.TimeWindow{
					Enabled:       true,
					ResetSchedule: opts.ResetSchedule,
					Timezone:      opts.Timezone,
				},
			}

			// Add min/max condition if specified in schema
			if prop.Minimum != nil {
				rule.Condition = fmt.Sprintf("payload.%s >= %f", paramName, *prop.Minimum)
			}

			rules = append(rules, rule)
		}

		// 2. Schema Enum Validation (e.g. Allowed Categories, Statuses, Currencies)
		if opts.StrictEnums && len(prop.Enum) > 0 {
			enumList := make([]string, len(prop.Enum))
			for i, e := range prop.Enum {
				enumList[i] = fmt.Sprintf("'%s'", e)
			}
			condition := fmt.Sprintf("payload.%s in [%s]", paramName, strings.Join(enumList, ", "))

			rules = append(rules, config.Rule{
				RuleID:      fmt.Sprintf("%s_%s_enum_guard", sanitizeID(tool.Name), sanitizeID(paramName)),
				ToolPattern: tool.Name,
				ValuePath:   fmt.Sprintf("$.%s", paramName),
				Accumulator: config.Accumulator{
					Type: config.AccumulatorCount,
				},
				Condition: condition,
			})
		}

		// 3. Schema Regex Pattern Validation
		if prop.Pattern != "" {
			rules = append(rules, config.Rule{
				RuleID:      fmt.Sprintf("%s_%s_pattern_guard", sanitizeID(tool.Name), sanitizeID(paramName)),
				ToolPattern: tool.Name,
				ValuePath:   fmt.Sprintf("$.%s", paramName),
				Accumulator: config.Accumulator{
					Type: config.AccumulatorCount,
				},
				Condition: fmt.Sprintf("payload.%s.matches('%s')", paramName, escapePattern(prop.Pattern)),
			})
		}

		// 4. Command / Query Injection Defense (Generic)
		if opts.EnableInjectionGuard && isExecutionVector(paramLower, prop.Description) {
			rules = append(rules, config.Rule{
				RuleID:      fmt.Sprintf("%s_%s_injection_guard", sanitizeID(tool.Name), sanitizeID(paramName)),
				ToolPattern: tool.Name,
				ValuePath:   fmt.Sprintf("$.%s", paramName),
				Accumulator: config.Accumulator{
					Type: config.AccumulatorCount,
				},
				Condition: fmt.Sprintf("!payload.%s.matches('(?i)(drop|delete|truncate|alter|grant|exec|rm\\\\s+-rf)')", paramName),
			})
		}

		// 5. Path Traversal Defense (Generic)
		if opts.EnablePathGuard && isPathVector(paramLower, prop.Description) {
			rules = append(rules, config.Rule{
				RuleID:      fmt.Sprintf("%s_%s_path_traversal_guard", sanitizeID(tool.Name), sanitizeID(paramName)),
				ToolPattern: tool.Name,
				ValuePath:   fmt.Sprintf("$.%s", paramName),
				Accumulator: config.Accumulator{
					Type: config.AccumulatorCount,
				},
				Condition: fmt.Sprintf("!payload.%s.contains('..')", paramName),
			})
		}
	}

	// Fallback: If no parameters generated specific rules, create a basic tool invocation rate limit
	if len(rules) == 0 {
		rules = append(rules, config.Rule{
			RuleID:      fmt.Sprintf("%s_rate_limit", sanitizeID(tool.Name)),
			ToolPattern: tool.Name,
			ValuePath:   "$",
			Accumulator: config.Accumulator{
				Type: config.AccumulatorCount,
			},
			TimeWindow: &config.TimeWindow{
				Enabled:       true,
				ResetSchedule: opts.ResetSchedule,
				Timezone:      opts.Timezone,
			},
		})
	}

	return rules
}

func isExecutionVector(paramName, description string) bool {
	combined := strings.ToLower(paramName + " " + description)
	keywords := []string{"query", "sql", "command", "cmd", "script", "code", "exec", "eval", "terminal", "bash"}
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func isPathVector(paramName, description string) bool {
	combined := strings.ToLower(paramName + " " + description)
	keywords := []string{"path", "file", "dir", "folder", "filename", "filepath", "location"}
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func sanitizeID(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	return reg.ReplaceAllString(name, "_")
}

func escapePattern(pat string) string {
	return strings.ReplaceAll(pat, `'`, `\'`)
}

func floatPtr(v float64) *float64 {
	return &v
}