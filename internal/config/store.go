package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Store manages thread-safe access to dynamic policy rules and handles persistence.
type Store struct {
	mu         sync.RWMutex
	policy     *Policy
	policyPath string
}

// NewStore constructs a thread-safe policy store loaded from a file path.
func NewStore(policyPath string) (*Store, error) {
	pol, err := LoadPolicy(policyPath)
	if err != nil {
		return nil, err
	}

	return &Store{
		policy:     pol,
		policyPath: policyPath,
	}, nil
}

// GetPolicy returns a thread-safe snapshot copy of the current policy rules.
func (s *Store) GetPolicy() *Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Deep copy rules to prevent race conditions during iteration
	rulesCopy := make([]Rule, len(s.policy.Rules))
	copy(rulesCopy, s.policy.Rules)

	return &Policy{
		Version: s.policy.Version,
		Rules:   rulesCopy,
	}
}

// UpdatePolicy validates and replaces the full policy in RAM and flushes to disk.
func (s *Store) UpdatePolicy(newPolicy *Policy) error {
	if err := newPolicy.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.policy = newPolicy
	return s.saveLocked()
}

// AddOrUpdateRule inserts a new rule or updates an existing rule in RAM and flushes to disk.
func (s *Store) AddOrUpdateRule(newRule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := false
	for i, r := range s.policy.Rules {
		if r.RuleID == newRule.RuleID {
			s.policy.Rules[i] = newRule
			updated = true
			break
		}
	}

	if !updated {
		s.policy.Rules = append(s.policy.Rules, newRule)
	}

	return s.saveLocked()
}

// DeleteRule removes a rule by ID from RAM and flushes to disk.
func (s *Store) DeleteRule(ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newRules := make([]Rule, 0, len(s.policy.Rules))
	found := false

	for _, r := range s.policy.Rules {
		if r.RuleID == ruleID {
			found = true
			continue
		}
		newRules = append(newRules, r)
	}

	if !found {
		return fmt.Errorf("rule_id '%s' not found", ruleID)
	}

	s.policy.Rules = newRules
	return s.saveLocked()
}

// saveLocked writes the current policy state to disk after validating. Assumes write lock is held.
func (s *Store) saveLocked() error {
	if err := s.policy.Validate(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.policy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal policy JSON: %w", err)
	}

	return os.WriteFile(s.policyPath, data, 0644)
}