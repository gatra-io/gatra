package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:   "gatra",
	Short: "GATRA: Trajectory Security Proxy & Policy Engine for AI Agents",
	Long: `GATRA (Guardrail Architecture for Trajectory-Restricted Agents)
A high-performance, sub-millisecond security proxy enforcing per-call,
cumulative, and CEL-based behavioral policy constraints on AI agent tool calls.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(genKeysCmd)
	rootCmd.AddCommand(validateConfigCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print GATRA version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GATRA Trajectory Proxy v%s\n", version)
	},
}