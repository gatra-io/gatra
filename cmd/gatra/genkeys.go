package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/gatra-io/gatra/internal/token"
)

var (
	genKeyTraj   string
	genKeyTool   string
	genKeyTTL    time.Duration
	pubOutFile   string
	privOutFile  string
	tokenOutFile string
	jsonOutput   bool
	noToken      bool
)

var genKeysCmd = &cobra.Command{
	Use:   "gen-keys",
	Short: "Generate Ed25519 keypair and mint scoped capability tokens",
	Run:   runGenKeys,
}

func init() {
	genKeysCmd.Flags().StringVarP(&genKeyTraj, "trajectory", "t", "session_001", "Trajectory ID claim bound to the capability token")
	genKeysCmd.Flags().StringVarP(&genKeyTool, "tool-pattern", "p", "*", "Tool pattern claim bound to the capability token")
	genKeysCmd.Flags().DurationVarP(&genKeyTTL, "ttl", "e", 24*time.Hour, "Token expiration TTL duration (e.g., 1h, 24h, 720h)")
	genKeysCmd.Flags().StringVar(&pubOutFile, "pub-out", "", "File path to write the Base64 Public Key")
	genKeysCmd.Flags().StringVar(&privOutFile, "priv-out", "", "File path to write the Base64 Private Key")
	genKeysCmd.Flags().StringVar(&tokenOutFile, "token-out", "", "File path to write the Base64 Capability Token")
	genKeysCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results in structured JSON format")
	genKeysCmd.Flags().BoolVar(&noToken, "no-token", false, "Generate keypair only without minting a capability token")
}

type KeyGenOutput struct {
	PublicKey  string                 `json:"public_key"`
	PrivateKey string                 `json:"private_key"`
	Token      string                 `json:"token,omitempty"`
	Claims     *token.CapabilityClaims `json:"claims,omitempty"`
}

func runGenKeys(cmd *cobra.Command, args []string) {
	keys, err := token.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating Ed25519 keypair: %v\n", err)
		os.Exit(1)
	}

	privBase64 := base64.StdEncoding.EncodeToString(keys.PrivateKey)
	pubBase64 := base64.StdEncoding.EncodeToString(keys.PublicKey)

	if pubOutFile != "" {
		if err := os.WriteFile(pubOutFile, []byte(pubBase64), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write public key file: %v\n", err)
			os.Exit(1)
		}
	}

	if privOutFile != "" {
		if err := os.WriteFile(privOutFile, []byte(privBase64), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write private key file: %v\n", err)
			os.Exit(1)
		}
	}

	var tokenStr string
	var claims *token.CapabilityClaims

	if !noToken {
		exp := time.Now().Add(genKeyTTL).Unix()
		claims = &token.CapabilityClaims{
			TrajectoryID: genKeyTraj,
			ToolPattern:  genKeyTool,
			ExpiresAt:    exp,
		}

		var err error
		tokenStr, err = token.MintToken(keys.PrivateKey, *claims)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error minting capability token: %v\n", err)
			os.Exit(1)
		}

		if tokenOutFile != "" {
			if err := os.WriteFile(tokenOutFile, []byte(tokenStr), 0600); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write token file: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if jsonOutput {
		out := KeyGenOutput{
			PublicKey:  pubBase64,
			PrivateKey: privBase64,
			Token:      tokenStr,
			Claims:     claims,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("🔑 GATRA Ed25519 Cryptographic Keypair")
	fmt.Println("==================================================")
	fmt.Printf("Public Key  (Base64) : %s\n", pubBase64)
	fmt.Printf("Private Key (Base64) : %s\n", privBase64)

	if !noToken {
		fmt.Println("\n🎟️ Capability Token")
		fmt.Println("--------------------------------------------------")
		fmt.Printf("Trajectory ID : %s\n", genKeyTraj)
		fmt.Printf("Tool Pattern  : %s\n", genKeyTool)
		fmt.Printf("Expires At    : %s (TTL: %s)\n", time.Unix(claims.ExpiresAt, 0).UTC().Format(time.RFC3339), genKeyTTL)
		fmt.Printf("Token         : %s\n", tokenStr)
	}
}