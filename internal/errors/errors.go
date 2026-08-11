package errors

import "errors"

var (
	// ErrPolicyViolation represents a general policy rejection.
	ErrPolicyViolation = errors.New("policy violation: tool execution rejected")

	// ErrPerCallLimitExceeded indicates a single transaction payload breached its limit.
	ErrPerCallLimitExceeded = errors.New("limit exceeded: single call payload out of bounds")

	// ErrCumulativeLimitExceeded indicates total trajectory balance for a session was exhausted.
	ErrCumulativeLimitExceeded = errors.New("limit exceeded: cumulative trajectory threshold reached")

	// ErrUniqueSetLimitExceeded indicates an agent attempted to touch too many distinct targets.
	ErrUniqueSetLimitExceeded = errors.New("cardinality limit exceeded: unique target capacity reached")

	// ErrInvalidToken indicates an expired or tampered capability token.
	ErrInvalidToken = errors.New("authentication failure: capability token is invalid or expired")

	// ErrInvalidConfiguration indicates malformed JSON or YAML rule definitions.
	ErrInvalidConfiguration = errors.New("configuration error: policy definition is malformed")
)

// ProxyError wraps an underlying error with an HTTP status code and rule identifier.
type ProxyError struct {
	Code   int
	RuleID string
	Err    error
}

func (e *ProxyError) Error() string {
	if e.RuleID != "" {
		return e.RuleID + ": " + e.Err.Error()
	}
	return e.Err.Error()
}

func (e *ProxyError) Unwrap() error {
	return e.Err
}

// NewProxyError constructs a typed proxy error.
func NewProxyError(code int, ruleID string, err error) *ProxyError {
	return &ProxyError{
		Code:   code,
		RuleID: ruleID,
		Err:    err,
	}
}