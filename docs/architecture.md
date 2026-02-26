# Architecture Overview

## System Overview

The iGaming Proxy is a production-grade Multi-Region Routing & Compliance-Aware Proxy Layer
for an iGaming Aggregator platform. It integrates multiple third-party game providers and
enforces jurisdiction-specific compliance rules transparently.

## Goals

- Route player traffic to the correct provider based on geographic location
- Enforce compliance rules per jurisdiction (UKGC, MGA, PAGCOR, LOCAL)
- Block restricted countries automatically
- Provide observability through structured logging and audit trails
- Support high availability via circuit breakers, retries, and health checks

## Component Descriptions

### Config Layer (`internal/config/`)
Loads all routing, provider, and compliance configuration from YAML files. Supports
environment variable overrides for secrets and runtime parameters.

### GeoIP Detector (`internal/geoip/`)
Uses MaxMind GeoLite2 database to resolve client IP addresses to country codes. Results
are cached in-memory with configurable TTL to reduce database lookups.

### Compliance Engine (`internal/compliance/`)
Evaluates whether a request from a given country is allowed, selects the appropriate
provider, and returns compliance flags. Uses pre-computed country→region maps for O(1)
lookups.

### License Validator (`internal/compliance/license.go`)
Verifies that a provider holds the required license for a given region/jurisdiction.

### Reverse Proxy Router (`internal/router/`)
Routes authenticated, compliance-approved requests to the appropriate upstream provider.
Implements per-provider circuit breakers, retry with exponential backoff, and provider
authentication (API key, Bearer token, HMAC).

### Middleware Stack (`internal/middleware/`)
A layered HTTP middleware chain:
1. **RequestID** - generates/propagates a unique request ID
2. **AuditLog** - records all request/response metadata
3. **RateLimiter** - per-IP token bucket rate limiting
4. **Auth** - API key or Bearer token validation
5. **RequestValidator** - Content-Type and body size validation
6. **GeoRouting** - resolves client IP to country code
7. **ComplianceCheck** - evaluates routing decision and compliance flags
8. **ProxyRouter** - executes the upstream request

### Audit Logger (`internal/audit/`)
Async JSON audit logger backed by a buffered channel. Supports stdout, file, and Kafka
(placeholder) output. Gracefully flushes on shutdown.

## Request Flow

```
Client Request
    │
    ▼
[RequestID Middleware]     -- generate/propagate X-Request-ID
    │
    ▼
[AuditLog Middleware]      -- record request start; log on completion
    │
    ▼
[RateLimiter]              -- per-IP token bucket; 429 if exceeded
    │
    ▼
[Auth Middleware]          -- validate API key or JWT Bearer token
    │
    ▼
[RequestValidator]         -- check Content-Type, body size limit
    │
    ▼
[GeoRouting]               -- resolve client IP → GeoResult (cached)
    │
    ▼
[ComplianceCheck]          -- evaluate RoutingDecision (allowed/blocked, provider, flags)
    │
    ▼
[ProxyRouter]              -- circuit breaker → build upstream request → retry → proxy
    │
    ▼
Upstream Provider
```

## Compliance Enforcement Model

Compliance is applied in two phases:

1. **Pre-routing** (ComplianceCheck middleware): Determines whether a request from a
   given country is allowed, which provider to use, and what compliance flags apply.

2. **Pre-proxy** (ProxyRouter): Validates that the selected provider holds the required
   license for the region before forwarding the request.

Compliance flags (e.g., `KYC_REQUIRED`, `GAMSTOP_CHECK`) are attached to the routing
decision and can be consumed by downstream systems.

## Provider Integration Model

Providers are configured in `configs/providers.yaml`. Each provider defines:
- Supported licenses and game types
- Base URL and health check endpoint
- Authentication method (API key, Bearer token, or HMAC)
- Timeout, retry, and circuit breaker settings

The proxy adds provider-specific authentication headers before forwarding requests.

## Deployment Architecture

### Docker Compose (Development)
- 3 gateway replicas behind a shared network
- Prometheus + Grafana for metrics and dashboards
- Redis for distributed rate limiting (future)

### Kubernetes / Helm (Production)
- HorizontalPodAutoscaler: 3–20 replicas based on CPU (70%) and memory (80%)
- RollingUpdate strategy: maxSurge 1, maxUnavailable 0
- Topology spread across availability zones
- Non-root security context (uid 1000)
- GeoIP database mounted from a PersistentVolumeClaim
- Config mounted from a ConfigMap

## Scaling Strategy

- **Horizontal scaling**: HPA scales pods based on CPU/memory pressure
- **Circuit breakers**: Prevent cascading failures to unhealthy providers
- **Rate limiting**: Per-IP token buckets prevent abuse
- **GeoIP caching**: In-memory cache with TTL reduces database contention

## Security Considerations

- All traffic authenticated via API key or JWT Bearer token
- Non-root container execution
- Secrets injected via environment variables (not baked into images)
- Audit trail for all requests with compliance metadata
- Blocked country list enforced at the compliance layer

## Monitoring and Observability

- Structured JSON logging via `slog`
- Async audit log with full request/response metadata
- `/healthz` endpoint for liveness
- `/readyz` endpoint for readiness (checks provider circuit breakers)
- Prometheus scrape endpoint (future)

## Future Expansion

- Distributed rate limiting via Redis
- Real JWT validation with JWKS endpoint
- Kafka audit log integration
- Prometheus metrics instrumentation
- WebSocket support for live game streaming
- Player session management integration
