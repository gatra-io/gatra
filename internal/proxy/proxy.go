package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gatra-io/gatra/internal/config"
	"github.com/gatra-io/gatra/internal/engine"
	"github.com/gatra-io/gatra/internal/evaluator"
	"github.com/gatra-io/gatra/internal/mcp"
	"github.com/gatra-io/gatra/internal/metrics"
	"github.com/gatra-io/gatra/internal/plugin"
	"github.com/gatra-io/gatra/internal/token"
)

// EphemeralDirective represents dynamic, task-level constraints passed via X-Gatra-Directive header
type EphemeralDirective struct {
	MaxPerCall    *float64 `json:"max_per_call,omitempty"`
	MaxCumulative *float64 `json:"max_cumulative,omitempty"`
	Condition     string   `json:"condition,omitempty"`
}

type Proxy struct {
	targetURL    *url.URL
	store        *config.Store
	engine       *engine.Engine
	reverseProxy *httputil.ReverseProxy
	logger       *slog.Logger
	keyring      *token.Keyring
	pluginMgr    *plugin.Manager
	dryRun       bool
}

func NewProxy(targetURL string, store *config.Store, eng *engine.Engine, log *slog.Logger, keyring *token.Keyring, pluginMgr *plugin.Manager, dryRun bool) (*Proxy, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(parsedURL)

	return &Proxy{
		targetURL:    parsedURL,
		store:        store,
		engine:       eng,
		reverseProxy: rp,
		logger:       log,
		keyring:      keyring,
		pluginMgr:    pluginMgr,
		dryRun:       dryRun,
	}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Simple Health Endpoint
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy", "engine": "gatra"})
		return
	}

	// 1. Mandatory Keyring Token Authentication
	tokenHeader := r.Header.Get("X-Capability-Token")
	if tokenHeader == "" {
		p.logger.Warn("request rejected: missing mandatory capability token header")
		p.writeErrorResponse(w, http.StatusUnauthorized, "auth_required", "missing mandatory X-Capability-Token header", "", "", start)
		return
	}

	claims, err := p.keyring.VerifyToken(tokenHeader)
	if err != nil {
		p.logger.Warn("request rejected: capability token verification failed across keyring", slog.String("error", err.Error()))
		p.writeErrorResponse(w, http.StatusUnauthorized, "auth_token_failed", err.Error(), "", "", start)
		return
	}

	trajectoryID := claims.TrajectoryID
	toolName := claims.ToolPattern

	// 2. Parse Ephemeral Task Directive (X-Gatra-Directive Header)
	directive := p.parseEphemeralDirective(r)
	if directive != nil {
		p.logger.Debug("ephemeral task directive intercepted",
			slog.String("trajectory_id", trajectoryID),
			slog.Any("directive", directive),
		)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeErrorResponse(w, http.StatusBadRequest, "global", "failed to read request payload body", trajectoryID, toolName, start)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	evalTargetBody := body

	// MCP JSON-RPC Auto-Detection
	if mcp.IsMCPPayload(body) {
		mcpTool, mcpArgs, err := mcp.UnpackMCP(body)
		if err != nil {
			p.logger.Warn("malformed MCP payload intercepted", slog.String("error", err.Error()))
			p.writeErrorResponse(w, http.StatusBadRequest, "mcp_parse_error", err.Error(), trajectoryID, toolName, start)
			return
		}

		if toolName == "" || toolName == "*" {
			toolName = mcpTool
		}

		evalTargetBody = mcpArgs
	}

	// Trigger Pre-Execute Plugin Hooks
	evt := &plugin.Event{
		Timestamp:    start,
		TrajectoryID: trajectoryID,
		ToolName:     toolName,
		Payload:      evalTargetBody,
	}

	if p.pluginMgr != nil {
		if err := p.pluginMgr.ExecutePre(evt); err != nil {
			p.logger.Warn("pre-execute plugin short-circuit", slog.String("error", err.Error()))
			p.writeErrorResponse(w, http.StatusForbidden, "plugin_pre_exec_failed", err.Error(), trajectoryID, toolName, start)
			return
		}
	}

	// Fetch snapshot of active rules from thread-safe store
	currentPolicy := p.store.GetPolicy()
	hasViolation := false

	for _, rule := range currentPolicy.Rules {
		if toolName != "" && !evaluator.MatchPattern(rule.ToolPattern, toolName) {
			continue
		}

		// Apply Monotonic Restriction Principle: Ephemeral directive can ONLY tighten limits
		effectiveRule := p.applyEphemeralLimits(rule, directive)

		// A. Evaluate Global CEL Policy Condition
		if effectiveRule.Condition != "" {
			passed, err := evaluator.EvaluateCondition(effectiveRule.Condition, evalTargetBody)
			if err != nil {
				p.logger.Warn("CEL condition evaluation failed",
					slog.String("rule_id", effectiveRule.RuleID),
					slog.String("error", err.Error()),
				)
				if p.dryRun {
					hasViolation = true
					p.recordDryRunViolation(effectiveRule.RuleID, "CEL evaluation error: "+err.Error(), trajectoryID, toolName, start)
				} else {
					p.writeErrorResponse(w, http.StatusBadRequest, effectiveRule.RuleID, "condition evaluation error: "+err.Error(), trajectoryID, toolName, start)
					return
				}
			} else if !passed {
				msg := "policy condition rejected request: " + effectiveRule.Condition
				if p.dryRun {
					hasViolation = true
					p.logger.Warn("🔍 [DRY-RUN AUDIT] CEL policy condition violation intercepted (request permitted)",
						slog.String("rule_id", effectiveRule.RuleID),
						slog.String("condition", effectiveRule.Condition),
					)
					p.recordDryRunViolation(effectiveRule.RuleID, msg, trajectoryID, toolName, start)
				} else {
					p.logger.Warn("CEL condition policy check rejected payload",
						slog.String("rule_id", effectiveRule.RuleID),
						slog.String("condition", effectiveRule.Condition),
					)
					p.writeErrorResponse(w, http.StatusForbidden, effectiveRule.RuleID, msg, trajectoryID, toolName, start)
					return
				}
			}
		}

		// B. Evaluate Ephemeral Directive CEL Condition (if specified)
		if directive != nil && directive.Condition != "" {
			passed, err := evaluator.EvaluateCondition(directive.Condition, evalTargetBody)
			if err != nil {
				p.logger.Warn("ephemeral directive condition evaluation failed",
					slog.String("rule_id", effectiveRule.RuleID),
					slog.String("error", err.Error()),
				)
				if p.dryRun {
					hasViolation = true
					p.recordDryRunViolation(effectiveRule.RuleID, "ephemeral condition evaluation error: "+err.Error(), trajectoryID, toolName, start)
				} else {
					p.writeErrorResponse(w, http.StatusBadRequest, effectiveRule.RuleID, "ephemeral directive condition error: "+err.Error(), trajectoryID, toolName, start)
					return
				}
			} else if !passed {
				msg := "ephemeral task directive condition rejected request: " + directive.Condition
				if p.dryRun {
					hasViolation = true
					p.logger.Warn("🔍 [DRY-RUN AUDIT] ephemeral task directive condition violation (permitted)",
						slog.String("rule_id", effectiveRule.RuleID),
						slog.String("condition", directive.Condition),
					)
					p.recordDryRunViolation(effectiveRule.RuleID, msg, trajectoryID, toolName, start)
				} else {
					p.logger.Warn("ephemeral task directive condition check rejected payload",
						slog.String("rule_id", effectiveRule.RuleID),
						slog.String("condition", directive.Condition),
					)
					p.writeErrorResponse(w, http.StatusForbidden, effectiveRule.RuleID, msg, trajectoryID, toolName, start)
					return
				}
			}
		}

		extracted, err := evaluator.ExtractValue(evalTargetBody, effectiveRule.ValuePath)
		if err != nil {
			p.logger.Warn("value extraction failed for matched rule",
				slog.String("rule_id", effectiveRule.RuleID),
				slog.String("error", err.Error()),
			)
			if p.dryRun {
				hasViolation = true
				p.recordDryRunViolation(effectiveRule.RuleID, "malformed payload: "+err.Error(), trajectoryID, toolName, start)
			} else {
				p.writeErrorResponse(w, http.StatusBadRequest, effectiveRule.RuleID, "malformed JSON payload or missing path: "+err.Error(), trajectoryID, toolName, start)
				return
			}
		}

		if err := evaluator.EvaluateRule(effectiveRule, extracted); err != nil {
			if p.dryRun {
				hasViolation = true
				p.logger.Warn("🔍 [DRY-RUN AUDIT] stateless limit violation intercepted (request permitted)",
					slog.String("rule_id", effectiveRule.RuleID),
					slog.String("error", err.Error()),
				)
				p.recordDryRunViolation(effectiveRule.RuleID, err.Error(), trajectoryID, toolName, start)
			} else {
				p.logger.Warn("stateless rule violation intercepted",
					slog.String("rule_id", effectiveRule.RuleID),
					slog.String("error", err.Error()),
				)
				p.writeErrorResponse(w, http.StatusForbidden, effectiveRule.RuleID, err.Error(), trajectoryID, toolName, start)
				return
			}
		}

		if err := p.engine.EvaluateAndIncrement(trajectoryID, effectiveRule, extracted); err != nil {
			if p.dryRun {
				hasViolation = true
				p.logger.Warn("🔍 [DRY-RUN AUDIT] stateful trajectory boundary intercepted (request permitted)",
					slog.String("rule_id", effectiveRule.RuleID),
					slog.String("error", err.Error()),
				)
				p.recordDryRunViolation(effectiveRule.RuleID, err.Error(), trajectoryID, toolName, start)
			} else {
				p.logger.Warn("stateful trajectory boundary intercepted",
					slog.String("rule_id", effectiveRule.RuleID),
					slog.String("error", err.Error()),
				)
				p.writeErrorResponse(w, http.StatusForbidden, effectiveRule.RuleID, err.Error(), trajectoryID, toolName, start)
				return
			}
		}
	}

	// Record Prometheus Telemetry for Approved / Permitted Request
	if !hasViolation {
		duration := time.Since(start).Seconds()
		metrics.RequestsTotal.WithLabelValues("approved", "none", toolName).Inc()
		metrics.EvaluationDuration.WithLabelValues("approved").Observe(duration)

		if p.pluginMgr != nil {
			evt.Status = "approved"
			evt.HTTPStatus = http.StatusOK
			p.pluginMgr.ExecutePost(evt)
		}

		p.logger.Info("request approved, forwarding to downstream target",
			slog.String("trajectory_id", trajectoryID),
			slog.String("tool_name", toolName),
		)
	} else {
		p.logger.Info("request permitted under dry-run mode, forwarding to downstream target",
			slog.String("trajectory_id", trajectoryID),
			slog.String("tool_name", toolName),
		)
	}

	p.reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) parseEphemeralDirective(r *http.Request) *EphemeralDirective {
	headerVal := r.Header.Get("X-Gatra-Directive")
	if headerVal == "" {
		return nil
	}

	var directive EphemeralDirective
	if err := json.Unmarshal([]byte(headerVal), &directive); err != nil {
		p.logger.Warn("failed to parse X-Gatra-Directive header JSON", slog.String("error", err.Error()))
		return nil
	}

	return &directive
}

func (p *Proxy) applyEphemeralLimits(rule config.Rule, directive *EphemeralDirective) config.Rule {
	if directive == nil {
		return rule
	}

	effectiveRule := rule

	// Monotonic Restriction: Ephemeral directive can ONLY tighten limits, never relax global policy
	if directive.MaxPerCall != nil {
		if effectiveRule.Limits.MaxPerCall == nil || *directive.MaxPerCall < *effectiveRule.Limits.MaxPerCall {
			effectiveRule.Limits.MaxPerCall = directive.MaxPerCall
		}
	}

	if directive.MaxCumulative != nil {
		if effectiveRule.Limits.MaxCumulative == nil || *directive.MaxCumulative < *effectiveRule.Limits.MaxCumulative {
			effectiveRule.Limits.MaxCumulative = directive.MaxCumulative
		}
	}

	return effectiveRule
}

func (p *Proxy) recordDryRunViolation(ruleID, msg string, trajectoryID, toolName string, start time.Time) {
	duration := time.Since(start).Seconds()

	metrics.RequestsTotal.WithLabelValues("dry_run_flagged", ruleID, toolName).Inc()
	metrics.EvaluationDuration.WithLabelValues("dry_run_flagged").Observe(duration)

	if p.pluginMgr != nil {
		p.pluginMgr.ExecutePost(&plugin.Event{
			Timestamp:    start,
			TrajectoryID: trajectoryID,
			ToolName:     toolName,
			Status:       "dry_run_flagged",
			RuleID:       ruleID,
			Reason:       msg,
			HTTPStatus:   http.StatusOK,
		})
	}
}

func (p *Proxy) writeErrorResponse(w http.ResponseWriter, status int, ruleID, msg string, trajectoryID, toolName string, start time.Time) {
	duration := time.Since(start).Seconds()

	metrics.RequestsTotal.WithLabelValues("rejected", ruleID, toolName).Inc()
	metrics.EvaluationDuration.WithLabelValues("rejected").Observe(duration)

	if p.pluginMgr != nil {
		p.pluginMgr.ExecutePost(&plugin.Event{
			Timestamp:    start,
			TrajectoryID: trajectoryID,
			ToolName:     toolName,
			Status:       "rejected",
			RuleID:       ruleID,
			Reason:       msg,
			HTTPStatus:   status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "rejected",
		"code":    status,
		"rule_id": ruleID,
		"error":   msg,
	})
}