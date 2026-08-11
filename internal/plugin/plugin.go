package plugin

import (
	"fmt"
	"time"
)

// Event contains execution metadata passed through pre- and post-execution plugin hooks.
type Event struct {
	Timestamp    time.Time `json:"timestamp"`
	TrajectoryID string    `json:"trajectory_id"`
	ToolName     string    `json:"tool_name"`
	Status       string    `json:"status"` // "approved" or "rejected"
	RuleID       string    `json:"rule_id,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	HTTPStatus   int       `json:"http_status"`
	Payload      []byte    `json:"payload,omitempty"`
}

// Plugin is the base interface that all extensions implement.
type Plugin interface {
	Name() string
}

// PreExecuteHook runs prior to policy evaluation. Returning an error short-circuits execution.
type PreExecuteHook interface {
	Plugin
	OnPreExecute(evt *Event) error
}

// PostExecuteHook runs asynchronously or synchronously after a request decision is finalized.
type PostExecuteHook interface {
	Plugin
	OnPostExecute(evt *Event)
}

// Manager manages plugin registration and executes hooks across the pipeline.
type Manager struct {
	plugins []Plugin
}

// NewManager constructs an empty plugin pipeline manager.
func NewManager() *Manager {
	return &Manager{
		plugins: make([]Plugin, 0),
	}
}

// Register attaches a new plugin implementation to the pipeline.
func (m *Manager) Register(p Plugin) {
	m.plugins = append(m.plugins, p)
}

// ExecutePre triggers all registered PreExecuteHook instances in sequence.
func (m *Manager) ExecutePre(evt *Event) error {
	for _, p := range m.plugins {
		if hook, ok := p.(PreExecuteHook); ok {
			if err := hook.OnPreExecute(evt); err != nil {
				return fmt.Errorf("plugin '%s' pre-execute failure: %w", p.Name(), err)
			}
		}
	}
	return nil
}

// ExecutePost triggers all registered PostExecuteHook instances in sequence.
func (m *Manager) ExecutePost(evt *Event) {
	for _, p := range m.plugins {
		if hook, ok := p.(PostExecuteHook); ok {
			hook.OnPostExecute(evt)
		}
	}
}