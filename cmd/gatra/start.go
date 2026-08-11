package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gatra-io/gatra/internal/admin"
	"github.com/gatra-io/gatra/internal/config"
	"github.com/gatra-io/gatra/internal/engine"
	"github.com/gatra-io/gatra/internal/logger"
	"github.com/gatra-io/gatra/internal/metrics"
	"github.com/gatra-io/gatra/internal/plugin"
	"github.com/gatra-io/gatra/internal/proxy"
	"github.com/gatra-io/gatra/internal/storage"
	"github.com/gatra-io/gatra/internal/token"
)

var (
	logLevel   string
	configPath string
	proxyAddr  string
	targetAddr string
	dbPath     string
	publicKey  string
	dryRun     bool
	sessionTTL time.Duration
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the GATRA security proxy and control plane",
	Run:   runStart,
}

func init() {
	startCmd.Flags().StringVarP(&logLevel, "log-level", "l", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")
	startCmd.Flags().StringVarP(&configPath, "config", "c", "policy.json", "Path to policy configuration JSON file")
	startCmd.Flags().StringVarP(&proxyAddr, "addr", "a", ":8080", "HTTP proxy listen address")
	startCmd.Flags().StringVarP(&targetAddr, "target", "t", "http://localhost:8081", "Downstream target backend service URL")
	startCmd.Flags().StringVarP(&dbPath, "db-path", "d", "gatra.db", "Path to embedded bbolt database file")
	startCmd.Flags().StringVarP(&publicKey, "public-key", "k", "", "Comma-separated Base64 Ed25519 Public Keys or key file paths (or set GATRA_PUBLIC_KEY env var)")
	startCmd.Flags().BoolVarP(&dryRun, "dry-run", "r", false, "Enable dry-run audit mode (log and record violations without blocking traffic)")
	startCmd.Flags().DurationVarP(&sessionTTL, "session-ttl", "s", 1*time.Hour, "Session state TTL duration for background cleanup")
}

func startMockBackendServer(addr string, log *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","message":"downstream execution completed"}`))
	})

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("mock backend server crashed", slog.String("error", err.Error()))
	}
}

func runStart(cmd *cobra.Command, args []string) {
	log := logger.Setup(logLevel)

	log.Info("initializing GATRA proxy engine",
		slog.String("version", version),
		slog.Int("pid", os.Getpid()),
		slog.Bool("dry_run_mode", dryRun),
	)

	pubKeyStr := publicKey
	if pubKeyStr == "" {
		pubKeyStr = os.Getenv("GATRA_PUBLIC_KEY")
	}

	if pubKeyStr == "" {
		log.Error("missing mandatory public key configuration: specify via --public-key (-k) flag or GATRA_PUBLIC_KEY environment variable")
		os.Exit(1)
	}

	keyring, err := token.NewKeyring(pubKeyStr)
	if err != nil {
		log.Error("failed to load Ed25519 public keyring", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("Ed25519 cryptographic keyring verification engine initialized",
		slog.Bool("zero_trust_auth", true),
		slog.Int("active_keys_count", keyring.KeyCount()),
	)

	store, err := storage.NewStore(dbPath)
	if err != nil {
		log.Error("failed to initialize storage engine", slog.String("error", err.Error()))
		os.Exit(1)
	}

	pluginMgr := plugin.NewManager()
	auditLogger := plugin.NewAuditLogger(log)
	pluginMgr.Register(auditLogger)
	log.Info("registered plugin pipeline extensions", slog.String("plugin", auditLogger.Name()))

	policyStore, err := config.NewStore(configPath)
	if err != nil {
		log.Error("failed to load policy configuration store", slog.String("error", err.Error()))
		os.Exit(1)
	}

	for _, rule := range policyStore.GetPolicy().Rules {
		twInfo := "disabled"
		if rule.TimeWindow != nil && rule.TimeWindow.Enabled {
			twInfo = fmt.Sprintf("schedule=%s, tz=%s", rule.TimeWindow.ResetSchedule, rule.TimeWindow.Timezone)
		}
		log.Debug("registered policy rule",
			slog.String("rule_id", rule.RuleID),
			slog.String("pattern", rule.ToolPattern),
			slog.String("time_window", twInfo),
		)
	}

	stateEngine, err := engine.NewEngine(sessionTTL, log, store)
	if err != nil {
		log.Error("failed to initialize state engine", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("stateful trajectory engine initialized", slog.Duration("session_ttl", sessionTTL))

	go startMockBackendServer(":8081", log)

	p, err := proxy.NewProxy(targetAddr, policyStore, stateEngine, log, keyring, pluginMgr, dryRun)
	if err != nil {
		log.Error("failed to initialize proxy handler", slog.String("error", err.Error()))
		os.Exit(1)
	}

	mainMux := http.NewServeMux()
	mainMux.Handle("/metrics", metrics.Handler())

	adminServer := admin.NewServer(policyStore, log)
	adminServer.RegisterRoutes(mainMux)

	mainMux.Handle("/", p)

	server := &http.Server{
		Addr:    proxyAddr,
		Handler: mainMux,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Info("GATRA proxy & control plane listening",
			slog.String("listen_addr", proxyAddr),
			slog.String("admin_ui", fmt.Sprintf("http://localhost%s/admin/ui", proxyAddr)),
			slog.String("metrics_url", fmt.Sprintf("http://localhost%s/metrics", proxyAddr)),
			slog.String("db_path", dbPath),
			slog.Bool("dry_run", dryRun),
		)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("proxy server stopped abruptly", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	sig := <-stopChan
	log.Info("received termination signal, starting graceful shutdown", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("error during HTTP server shutdown", slog.String("error", err.Error()))
	} else {
		log.Info("HTTP server shut down cleanly")
	}

	if err := store.Close(); err != nil {
		log.Error("error closing bbolt storage", slog.String("error", err.Error()))
	} else {
		log.Info("embedded storage flushed and unlocked cleanly")
	}

	log.Info("GATRA proxy stopped successfully")
}