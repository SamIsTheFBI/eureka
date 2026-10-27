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

	"eureka/services/gateway/internal/admin"
	"eureka/services/gateway/internal/agent"
	"eureka/services/gateway/internal/aggregate"
	"eureka/services/gateway/internal/audit"
	"eureka/services/gateway/internal/auth"
	"eureka/services/gateway/internal/config"
	"eureka/services/gateway/internal/crypto"
	"eureka/services/gateway/internal/db"
	"eureka/services/gateway/internal/mcpfront"
	"eureka/services/gateway/internal/metrics"
	"eureka/services/gateway/internal/router"
	"eureka/services/gateway/internal/upstream"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("connecting to database", "dsn", cfg.DatabaseURL)
	database, err := db.Connect(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	if err := database.Migrate(); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	// Bootstrap default admin user
	bootstrapAdmin(database, cfg, logger)

	// Seed default upstreams
	seedUpstreams(database, cfg, logger)

	// Subsystems initialization
	authSvc := auth.NewService(database)
	upstreamMgr := upstream.NewManager(database, cfg.MasterKey, logger)
	if err := upstreamMgr.Start(context.Background()); err != nil {
		logger.Error("failed to start upstream manager", "error", err)
	}

	aggregator := aggregate.NewAggregator(upstreamMgr)
	auditWriter := audit.NewWriter(database, logger)
	callRouter := router.NewRouter(upstreamMgr, auditWriter, logger)
	chatAgent := agent.NewAgent(cfg, database, aggregator, callRouter, logger)
	mcpFrontHandler := mcpfront.NewHandler(authSvc, aggregator, callRouter, auditWriter, logger)
	adminServer := admin.NewServer(cfg, database, authSvc, upstreamMgr, aggregator, callRouter, chatAgent, logger)

	// Root HTTP router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Health and readiness endpoints
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := database.PingContext(ctx); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// Public Unified MCP Endpoint
	r.Handle("/mcp", mcpFrontHandler)

	// Admin and Dashboard REST APIs
	adminServer.RegisterRoutes(r)

	// Start Prometheus metrics server
	metricsSrv := metrics.StartServer(cfg.MetricsPort)
	logger.Info("metrics listening", "port", cfg.MetricsPort)

	// Gateway HTTP Server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("gateway server listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down gateway gracefully (draining up to 30s)")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = server.Shutdown(ctx)
	_ = metricsSrv.Shutdown(ctx)
	upstreamMgr.Stop()
	auditWriter.Close()
	fmt.Println("gateway stopped cleanly")
}

func bootstrapAdmin(db *db.DB, cfg *config.Config, logger *slog.Logger) {
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", cfg.AdminEmail).Scan(&count)
	if count == 0 {
		hash, err := crypto.HashPassword(cfg.AdminPassword)
		if err != nil {
			logger.Error("failed to hash admin password", "error", err)
			return
		}
		_, err = db.Exec("INSERT INTO users (email, password_hash, role) VALUES ($1, $2, 'admin')", cfg.AdminEmail, hash)
		if err != nil {
			logger.Error("failed to bootstrap admin user", "error", err)
		} else {
			logger.Info("bootstrapped admin account", "email", cfg.AdminEmail)
		}
	}
}

func seedUpstreams(db *db.DB, cfg *config.Config, logger *slog.Logger) {
	type seed struct {
		slug       string
		name       string
		url        string
		authKind   string
		authHeader string
		cred       string
	}

	seeds := []seed{
		{
			slug:       "woo",
			name:       "WooCommerce",
			url:        "http://woo-mcp.store.svc.cluster.local:8080/mcp",
			authKind:   "bearer",
			authHeader: "Authorization",
			cred:       os.Getenv("WOO_MCP_TOKEN"),
		},
		{
			slug:       "utils",
			name:       "Utils",
			url:        "http://utils-mcp.platform.svc.cluster.local:8080/mcp",
			authKind:   "header",
			authHeader: "X-Utils-Key",
			cred:       "utils-secret-key-12345",
		},
		{
			slug:     "docs",
			name:     "DeepWiki Docs",
			url:      "https://mcp.deepwiki.com/mcp",
			authKind: "none",
		},
	}

	for _, s := range seeds {
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM mcp_servers WHERE slug = $1", s.slug).Scan(&count)
		if count == 0 {
			credEnc := ""
			if s.cred != "" {
				enc, err := crypto.EncryptCredential(cfg.MasterKey, s.cred, s.slug)
				if err == nil {
					credEnc = enc
				}
			}
			_, err := db.Exec(`
				INSERT INTO mcp_servers (slug, name, url, enabled, auth_kind, auth_header, credential_enc, call_timeout_ms)
				VALUES ($1, $2, $3, true, $4, $5, $6, 30000)
			`, s.slug, s.name, s.url, s.authKind, s.authHeader, credEnc)
			if err != nil {
				logger.Error("failed to seed upstream", "slug", s.slug, "error", err)
			} else {
				logger.Info("seeded upstream server", "slug", s.slug)
			}
		}
	}
}
