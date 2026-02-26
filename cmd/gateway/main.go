package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"igaming-proxy/internal/audit"
	"igaming-proxy/internal/compliance"
	"igaming-proxy/internal/config"
	"igaming-proxy/internal/geoip"
	"igaming-proxy/internal/middleware"
	"igaming-proxy/internal/router"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "configs"
	}

	cfg, err := config.Load(configDir)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize GeoIP detector
	detector, err := geoip.NewDetector(cfg.GeoIP.DatabasePath, cfg.GeoIP.CacheTTLMin)
	if err != nil {
		logger.Warn("GeoIP database unavailable, using fallback", "error", err)
		detector = nil
	}
	if detector != nil {
		defer detector.Close()
	}

	// Initialize compliance engine
	engine := compliance.NewEngine(cfg.RoutingRules, cfg.Providers, cfg.Compliance)

	// Initialize audit logger
	auditLogger, err := audit.NewLogger(cfg.Audit.OutputType, cfg.Audit.FilePath, logger)
	if err != nil {
		logger.Error("failed to initialize audit logger", "error", err)
		os.Exit(1)
	}

	// Initialize proxy router
	proxyRouter := router.NewProxy(cfg.Providers, engine, logger)

	// Initialize rate limiter
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst)

	// Build middleware chain for /api/v1/*
	apiHandler := buildMiddlewareChain(
		proxyRouter,
		cfg,
		detector,
		engine,
		auditLogger,
		rateLimiter,
		logger,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.HandleFunc("GET /readyz", readyzHandler(proxyRouter, logger))
	mux.Handle("/api/v1/", apiHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting gateway", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gateway")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}

	auditLogger.Flush()
	logger.Info("gateway stopped")
}

// buildMiddlewareChain assembles the full middleware stack.
func buildMiddlewareChain(
	h http.Handler,
	cfg *config.Config,
	detector *geoip.Detector,
	engine *compliance.Engine,
	auditLogger *audit.Logger,
	rateLimiter *middleware.RateLimiter,
	logger *slog.Logger,
) http.Handler {
	// Apply middlewares from innermost to outermost
	h = middleware.NewComplianceCheck(engine)(h)

	if detector != nil {
		h = middleware.NewGeoRouting(detector)(h)
	} else {
		h = noopGeoRouting(h)
	}

	h = middleware.NewRequestValidator()(h)
	h = middleware.NewAuth(cfg.Auth.JWTSecret, cfg.Auth.JWTSecret)(h)
	h = middleware.NewRateLimit(rateLimiter)(h)
	h = middleware.NewAuditLog(auditLogger, logger)(h)
	h = middleware.NewRequestID(h)
	return h
}

// noopGeoRouting is a fallback when GeoIP is unavailable.
func noopGeoRouting(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func readyzHandler(proxyRouter *router.Proxy, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		status := proxyRouter.ReadyzCheck()
		allReady := true
		for _, ready := range status {
			if !ready {
				allReady = false
				break
			}
		}

		code := http.StatusOK
		if !allReady {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if err := json.NewEncoder(w).Encode(status); err != nil {
			logger.Error("failed to encode readyz response", "error", err)
		}
	}
}
