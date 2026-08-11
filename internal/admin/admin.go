package admin

import (
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gatra-io/gatra/internal/config"
)

//go:embed ui/index.html
var uiFS embed.FS

type Server struct {
	store  *config.Store
	logger *slog.Logger
}

func NewServer(store *config.Store, logger *slog.Logger) *Server {
	return &Server{
		store:  store,
		logger: logger,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/ui", s.handleUI)
	mux.HandleFunc("/admin/api/policies", s.handlePoliciesAPI)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	content, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "Failed to load embedded UI assets", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) handlePoliciesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		pol := s.store.GetPolicy()
		_ = json.NewEncoder(w).Encode(pol)

	case http.MethodPost:
		var newRule config.Rule
		if err := json.NewDecoder(r.Body).Decode(&newRule); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON rule body: " + err.Error()})
			return
		}

		if newRule.RuleID == "" || newRule.ToolPattern == "" || newRule.ValuePath == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rule_id, tool_pattern, and value_path are required"})
			return
		}

		if err := s.store.AddOrUpdateRule(newRule); err != nil {
			s.logger.Error("failed to update policy store", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		s.logger.Info("🔥 policy rule hot-reloaded dynamically into RAM & persisted to disk",
			slog.String("rule_id", newRule.RuleID),
			slog.String("tool_pattern", newRule.ToolPattern),
		)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "rule deployed and hot-reloaded"})

	case http.MethodDelete:
		ruleID := r.URL.Query().Get("rule_id")
		if ruleID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing rule_id query parameter"})
			return
		}

		if err := s.store.DeleteRule(ruleID); err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		s.logger.Info("🔥 policy rule deleted dynamically from RAM & disk", slog.String("rule_id", ruleID))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "rule deleted"})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}