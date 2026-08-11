package plugin

import (
	"log/slog"
)

// AuditLoggerPlugin captures execution outcome events and logs security audit records.
type AuditLoggerPlugin struct {
	logger *slog.Logger
}

// NewAuditLogger constructs an AuditLoggerPlugin.
func NewAuditLogger(log *slog.Logger) *AuditLoggerPlugin {
	return &AuditLoggerPlugin{
		logger: log,
	}
}

func (a *AuditLoggerPlugin) Name() string {
	return "audit-logger"
}

// OnPostExecute logs security audit records for blocked and approved execution events.
func (a *AuditLoggerPlugin) OnPostExecute(evt *Event) {
	if evt.Status == "rejected" {
		a.logger.Warn("🚨 SECURITY AUDIT EVENT: tool execution blocked",
			slog.String("trajectory_id", evt.TrajectoryID),
			slog.String("tool_name", evt.ToolName),
			slog.String("rule_id", evt.RuleID),
			slog.String("reason", evt.Reason),
			slog.Int("http_status", evt.HTTPStatus),
		)
	} else {
		a.logger.Info("AUDIT EVENT: tool execution approved",
			slog.String("trajectory_id", evt.TrajectoryID),
			slog.String("tool_name", evt.ToolName),
			slog.Int("http_status", evt.HTTPStatus),
		)
	}
}