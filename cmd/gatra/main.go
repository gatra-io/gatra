package main

import (
	"os"
)

func main() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(genKeysCmd)
	rootCmd.AddCommand(validateConfigCmd)
	rootCmd.AddCommand(discoverCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}