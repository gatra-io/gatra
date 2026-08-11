package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gatra-io/gatra/internal/config"
)

var valConfigPath string

var validateConfigCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validate policy configuration file syntax and schema rules",
	Run:   runValidateConfig,
}

func init() {
	validateConfigCmd.Flags().StringVarP(&valConfigPath, "config", "c", "policy.json", "Path to policy configuration file to validate")
}

func runValidateConfig(cmd *cobra.Command, args []string) {
	pol, err := config.LoadPolicy(valConfigPath)
	if err != nil {
		fmt.Printf("❌ Policy Configuration Error: %v\n", err)
		return
	}

	fmt.Printf("✓ Policy configuration file '%s' is valid!\n", valConfigPath)
	fmt.Printf("  Version: %s\n", pol.Version)
	fmt.Printf("  Loaded Rules: %d\n", len(pol.Rules))

	for i, rule := range pol.Rules {
		fmt.Printf("  [%d] Rule ID: %s | Pattern: %s | Path: %s\n", i+1, rule.RuleID, rule.ToolPattern, rule.ValuePath)
	}
}