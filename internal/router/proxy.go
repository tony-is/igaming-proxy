package router

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"igaming-proxy/internal/compliance"
	"igaming-proxy/internal/config"
)

// Context keys used by the proxy.
type contextKey string

const (
	ContextKeyGeo       contextKey = "geo"
	ContextKeyDecision  contextKey = "decision"
	ContextKeyRequestID contextKey = "request_id"
)

// circuitBreaker tracks provider failures and implements the circuit breaker pattern.
type circuitBreaker struct {
	failures    atomic.Int64
	threshold   int64
	timeout     time.Duration
	openedAt    atomic.Int64 // Unix nanoseconds
	mu          sync.Mutex
}

func newCircuitBreaker(threshold int, timeoutSeconds int) *circuitBreaker {
	return &circuitBreaker{
		threshold: int64(threshold),
		timeout:   time.Duration(timeoutSeconds) * time.Second,
	}
}

// isAvailable returns true if the circuit is closed or half-open.
func (cb *circuitBreaker) isAvailable() bool {
	if cb.failures.Load() < cb.threshold {
		return true
	}
	openedAt := time.Unix(0, cb.openedAt.Load())
	if time.Since(openedAt) > cb.timeout {
		// Half-open: allow one request through
		cb.mu.Lock()
		defer cb.mu.Unlock()
		if cb.failures.Load() >= cb.threshold && time.Since(openedAt) > cb.timeout {
			cb.failures.Store(0)
			return true
		}
	}
	return false
}

func (cb *circuitBreaker) recordFailure() {
	count := cb.failures.Add(1)
	if count >= cb.threshold {
		cb.openedAt.CompareAndSwap(0, time.Now().UnixNano())
	}
}

func (cb *circuitBreaker) recordSuccess() {
	cb.failures.Store(0)
	cb.openedAt.Store(0)
}

// Proxy is the reverse proxy that routes requests to upstream providers.
type Proxy struct {
	providers       map[string]config.Provider
	breakers        map[string]*circuitBreaker
	licenseValidator *compliance.Engine
	client          *http.Client
	logger          *slog.Logger
}

// NewProxy creates a Proxy and starts background health checks.
func NewProxy(providersConfig config.ProvidersConfig, engine *compliance.Engine, logger *slog.Logger) *Proxy {
	providerMap := make(map[string]config.Provider, len(providersConfig.Providers))
	breakers := make(map[string]*circuitBreaker, len(providersConfig.Providers))

	for _, p := range providersConfig.Providers {
		providerMap[p.ID] = p
		breakers[p.ID] = newCircuitBreaker(p.CircuitBreaker.Threshold, p.CircuitBreaker.TimeoutSeconds)
	}

	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	}

	p := &Proxy{
		providers:        providerMap,
		breakers:         breakers,
		licenseValidator: engine,
		client:           &http.Client{Transport: transport},
		logger:           logger,
	}

	go p.healthCheckLoop()
	return p
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	decision := getDecision(r.Context())
	if decision == nil || !decision.Allowed {
		reason := "blocked"
		if decision != nil {
			reason = decision.BlockReason
		}
		http.Error(w, reason, http.StatusForbidden)
		return
	}

	cb, ok := p.breakers[decision.ProviderID]
	if ok && !cb.isAvailable() {
		http.Error(w, "service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	if !p.licenseValidator.ValidateProviderLicense(decision.ProviderID, decision.License) {
		http.Error(w, "provider not licensed for this region", http.StatusForbidden)
		return
	}

	// Build upstream URL
	upstreamPath := strings.TrimPrefix(r.URL.Path, "/api/v1")
	upstreamURL := decision.ProviderBaseURL + upstreamPath
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	provider := p.providers[decision.ProviderID]
	var lastErr error
	for attempt := 0; attempt <= provider.RetryCount; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			time.Sleep(backoff)
		}

		if err := p.doRequest(w, r, upstreamURL, provider, decision); err != nil {
			lastErr = err
			if cb != nil {
				cb.recordFailure()
			}
			p.logger.Warn("upstream request failed", "attempt", attempt+1, "error", err)
			continue
		}

		if cb != nil {
			cb.recordSuccess()
		}
		return
	}

	p.logger.Error("all upstream attempts failed", "provider", decision.ProviderID, "error", lastErr)
	http.Error(w, "bad gateway", http.StatusBadGateway)
}

func (p *Proxy) doRequest(w http.ResponseWriter, r *http.Request, upstreamURL string, provider config.Provider, decision *compliance.RoutingDecision) error {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		return fmt.Errorf("creating upstream request: %w", err)
	}

	// Copy safe headers
	skipHeaders := map[string]bool{
		"Host":            true,
		"Connection":      true,
		"X-Forwarded-For": true,
		"X-Real-Ip":       true,
	}
	for k, vs := range r.Header {
		if !skipHeaders[k] {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	// Add provider authentication
	switch provider.Auth.Type {
	case "api_key":
		req.Header.Set(provider.Auth.Header, provider.Auth.Value)
	case "bearer_token":
		req.Header.Set(provider.Auth.Header, "Bearer "+provider.Auth.Value)
	case "hmac":
		req.Header.Set(provider.Auth.Header, computeHMAC(r, provider.Auth.Value))
	}

	// Add compliance metadata headers
	req.Header.Set("X-Region", decision.Region)
	req.Header.Set("X-License", decision.License)
	if rid := getRequestID(r.Context()); rid != "" {
		req.Header.Set("X-Request-ID", rid)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

// computeHMAC computes an HMAC-SHA256 signature of the request method and path.
func computeHMAC(r *http.Request, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(r.Method + "\n" + r.URL.RequestURI()))
	return hex.EncodeToString(mac.Sum(nil))
}

func getDecision(ctx context.Context) *compliance.RoutingDecision {
	if v, ok := ctx.Value(ContextKeyDecision).(*compliance.RoutingDecision); ok {
		return v
	}
	// Also check middleware context key
	type mwKey string
	if v, ok := ctx.Value(mwKey("decision")).(*compliance.RoutingDecision); ok {
		return v
	}
	return nil
}

func getRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return v
	}
	type mwKey string
	if v, ok := ctx.Value(mwKey("request_id")).(string); ok {
		return v
	}
	return ""
}

func (p *Proxy) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for id, provider := range p.providers {
			go func(id string, provider config.Provider) {
				healthURL := provider.BaseURL + provider.HealthCheck
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
				if err != nil {
					return
				}
				resp, err := p.client.Do(req)
				if err != nil {
					p.logger.Warn("provider health check failed", "provider", id, "error", err)
					return
				}
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					if cb, ok := p.breakers[id]; ok {
						cb.recordSuccess()
					}
				}
			}(id, provider)
		}
	}
}

// ReadyzCheck performs a readiness check by verifying provider health.
func (p *Proxy) ReadyzCheck() map[string]bool {
	results := make(map[string]bool)
	for id, cb := range p.breakers {
		results[id] = cb.isAvailable()
	}
	return results
}


