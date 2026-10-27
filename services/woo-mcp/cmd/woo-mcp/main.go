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

	"eureka/services/woo-mcp/internal/mcp"
	"eureka/services/woo-mcp/internal/tools"
	"eureka/services/woo-mcp/internal/woo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mcpToken := os.Getenv("MCP_TOKEN")
	if mcpToken == "" {
		mcpToken = os.Getenv("AUTH_TOKEN")
	}

	wooBaseURL := os.Getenv("WOO_BASE_URL")
	if wooBaseURL == "" {
		wooBaseURL = "http://wordpress.store.svc.cluster.local:8080"
	}
	wooConsumerKey := os.Getenv("WOO_CONSUMER_KEY")
	wooConsumerSecret := os.Getenv("WOO_CONSUMER_SECRET")
	wooHostHeader := os.Getenv("WOO_HOST_HEADER")
	if wooHostHeader == "" {
		wooHostHeader = "store.localtest.me"
	}

	wooClient := woo.NewClient(wooBaseURL, wooConsumerKey, wooConsumerSecret, wooHostHeader)
	registry := tools.NewRegistry(wooClient)
	mcpHandler := mcp.NewHandler(registry, mcpToken, logger)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting woo-mcp server", "port", port, "woo_url", wooBaseURL, "woo_host", wooHostHeader)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down woo-mcp gracefully")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
	fmt.Println("server stopped")
}
