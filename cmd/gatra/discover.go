package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gatra-io/gatra/internal/discovery"
)

var (
	mcpSchemaFile string
	outputPolicy  string
	stdoutMode    bool

	// Enterprise Configurable Options
	maxPerCall    float64
	maxCumulative float64
	resetSchedule string
	timezone      string
	noInjection   bool
	noPathGuard   bool
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Automatically discover MCP tools and generate schema-driven security policies",
	Run:   runDiscover,
}

func init() {
	discoverCmd.Flags().StringVarP(&mcpSchemaFile, "schema", "s", "", "Path to MCP tools manifest JSON or OpenAPI schema file (required)")
	discoverCmd.Flags().StringVarP(&outputPolicy, "out", "o", "policy.json", "Output path for auto-generated policy JSON file")
	discoverCmd.Flags().BoolVar(&stdoutMode, "stdout", false, "Print generated policy directly to standard output")

	discoverCmd.Flags().Float64Var(&maxPerCall, "default-max-call", 100.00, "Default single-call numeric limit when unspecified in schema")
	discoverCmd.Flags().Float64Var(&maxCumulative, "default-max-cumul", 1000.00, "Default cumulative trajectory boundary when unspecified in schema")
	discoverCmd.Flags().StringVar(&resetSchedule, "reset-schedule", "@daily", "Time-window reset schedule (@hourly, @daily, @weekly)")
	discoverCmd.Flags().StringVar(&timezone, "timezone", "UTC", "Timezone for reset schedules")
	discoverCmd.Flags().BoolVar(&noInjection, "no-injection-guard", false, "Disable automatic command/SQL injection checks")
	discoverCmd.Flags().BoolVar(&noPathGuard, "no-path-guard", false, "Disable automatic directory traversal checks")

	_ = discoverCmd.MarkFlagRequired("schema")
}

func runDiscover(cmd *cobra.Command, args []string) {
	fmt.Printf("🔍 Analyzing MCP Tool Manifest: %s...\n", mcpSchemaFile)

	opts := discovery.Options{
		DefaultMaxPerCall:    maxPerCall,
		DefaultMaxCumulative: maxCumulative,
		ResetSchedule:        resetSchedule,
		Timezone:             timezone,
		EnableInjectionGuard: !noInjection,
		EnablePathGuard:      !noPathGuard,
		StrictEnums:          true,
	}

	policy, err := discovery.GeneratePolicyFromMCP(mcpSchemaFile, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Discovery Failed: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to format policy JSON: %v\n", err)
		os.Exit(1)
	}

	if stdoutMode {
		fmt.Println(string(data))
		return
	}

	if err := os.WriteFile(outputPolicy, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to write policy file to '%s': %v\n", outputPolicy, err)
		os.Exit(1)
	}

	fmt.Println("==================================================")
	fmt.Printf("✓ Policy Auto-Discovered & Generated Successfully!\n")
	fmt.Printf("  Target File    : %s\n", outputPolicy)
	fmt.Printf("  Rules Drafted  : %d\n", len(policy.Rules))
	fmt.Println("--------------------------------------------------")
	for i, r := range policy.Rules {
		condStr := r.Condition
		if condStr == "" {
			condStr = "(none)"
		}
		fmt.Printf("  [%d] Rule ID: %-32s | Pattern: %-16s | Condition: %s\n", i+1, r.RuleID, r.ToolPattern, condStr)
	}
	fmt.Println("==================================================")
}