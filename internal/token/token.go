package token

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type CapabilityClaims struct {
	TrajectoryID string `json:"traj_id"`
	ToolPattern  string `json:"tool"`
	ExpiresAt    int64  `json:"exp"`
}

type SignedTokenPayload struct {
	Payload   CapabilityClaims `json:"payload"`
	Signature string           `json:"signature"`
}

type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// Keyring manages multiple Ed25519 public keys to enable zero-downtime key rotation.
type Keyring struct {
	keys []ed25519.PublicKey
}

func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 keypair: %w", err)
	}
	return &KeyPair{PublicKey: pub, PrivateKey: priv}, nil
}

// LoadPublicKey parses an Ed25519 public key from a raw Base64 string or a file path.
func LoadPublicKey(input string) (ed25519.PublicKey, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("public key input cannot be empty")
	}

	raw := input
	if _, err := os.Stat(input); err == nil {
		content, err := os.ReadFile(input)
		if err != nil {
			return nil, fmt.Errorf("failed to read public key file '%s': %w", input, err)
		}
		raw = strings.TrimSpace(string(content))
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encoding for public key: %w", err)
	}

	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 public key size: expected %d bytes, got %d", ed25519.PublicKeySize, len(decoded))
	}

	return ed25519.PublicKey(decoded), nil
}

// NewKeyring constructs a Keyring from comma-separated key strings or file paths.
func NewKeyring(inputs ...string) (*Keyring, error) {
	var keys []ed25519.PublicKey

	for _, input := range inputs {
		parts := strings.Split(input, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			pk, err := LoadPublicKey(part)
			if err != nil {
				return nil, fmt.Errorf("failed to load key in keyring: %w", err)
			}
			keys = append(keys, pk)
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("keyring must contain at least one valid public key")
	}

	return &Keyring{keys: keys}, nil
}

// KeyCount returns the total number of active public keys registered in the keyring.
func (k *Keyring) KeyCount() int {
	return len(k.keys)
}

func MintToken(privKey ed25519.PrivateKey, claims CapabilityClaims) (string, error) {
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal capability claims: %w", err)
	}

	signature := ed25519.Sign(privKey, payloadBytes)

	st := SignedTokenPayload{
		Payload:   claims,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}

	finalBytes, err := json.Marshal(st)
	if err != nil {
		return "", fmt.Errorf("failed to marshal signed token: %w", err)
	}

	return base64.StdEncoding.EncodeToString(finalBytes), nil
}

func VerifyToken(pubKey ed25519.PublicKey, tokenStr string) (*CapabilityClaims, error) {
	rawBytes, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 token encoding: %w", err)
	}

	var st SignedTokenPayload
	if err := json.Unmarshal(rawBytes, &st); err != nil {
		return nil, fmt.Errorf("malformed token payload structure: %w", err)
	}

	if st.Payload.ExpiresAt > 0 && time.Now().Unix() > st.Payload.ExpiresAt {
		return nil, fmt.Errorf("capability token has expired")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(st.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature base64 encoding: %w", err)
	}

	payloadBytes, err := json.Marshal(st.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct payload bytes: %w", err)
	}

	if !ed25519.Verify(pubKey, payloadBytes, sigBytes) {
		return nil, fmt.Errorf("Ed25519 signature verification failed")
	}

	return &st.Payload, nil
}

// VerifyToken verifies incoming capability tokens against all registered keys in the Keyring.
func (k *Keyring) VerifyToken(tokenStr string) (*CapabilityClaims, error) {
	var lastErr error
	for _, pubKey := range k.keys {
		claims, err := VerifyToken(pubKey, tokenStr)
		if err == nil {
			return claims, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("keyring token verification failed across %d registered keys: %w", len(k.keys), lastErr)
}