# iGaming Proxy

Multi-Region Routing & Compliance-Aware Proxy Layer for an iGaming Aggregator Platform.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Request                        │
└────────────────────────────┬────────────────────────────────┘
                             │
                    ┌────────▼─────────┐
                    │   RequestID MW   │
                    └────────┬─────────┘
                    ┌────────▼─────────┐
                    │   AuditLog MW    │
                    └────────┬─────────┘
                    ┌────────▼─────────┐
                    │  RateLimiter MW  │
                    └────────┬─────────┘
                    ┌────────▼─────────┐
                    │     Auth MW      │
                    └────────┬─────────┘
                    ┌────────▼─────────┐
                    │ RequestValidator │
                    └────────┬─────────┘
                    ┌────────▼─────────┐
                    │  GeoRouting MW   │ ◄─ MaxMind GeoIP
                    └────────┬─────────┘
                    ┌────────▼─────────┐
                    │ ComplianceCheck  │ ◄─ Routing + Compliance Rules
                    └────────┬─────────┘
                    ┌────────▼─────────┐
                    │   ProxyRouter    │ ◄─ Circuit Breaker + Retry
                    └────────┬─────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
   ┌──────▼──────┐  ┌───────▼──────┐  ┌────────▼────────┐
   │ Evolution   │  │  Pragmatic   │  │    Playtech /   │
   │  Gaming     │  │    Play      │  │  Asia Gaming    │
   └─────────────┘  └──────────────┘  └─────────────────┘
```

## Features

- **Config-driven routing**: All routing rules defined in YAML — no hardcoding
- **Multi-jurisdiction compliance**: UKGC, MGA, PAGCOR, LOCAL license enforcement
- **Automatic geo-blocking**: Blocked countries return 403 immediately
- **Circuit breakers**: Per-provider failure detection with automatic recovery
- **Retry with backoff**: Exponential backoff on upstream failures
- **Rate limiting**: Per-IP token bucket (100 RPS / 200 burst by default)
- **Authentication**: API key or JWT Bearer token
- **Audit logging**: Async structured JSON audit trail
- **Graceful shutdown**: 30-second drain on SIGINT/SIGTERM
- **Health/readiness probes**: `/healthz` and `/readyz` endpoints

## Quick Start

### Docker

```bash
docker-compose -f deployments/docker-compose.yaml up
```

### Helm

```bash
helm upgrade --install igaming-proxy deployments/helm/igaming-proxy \
  --set config.jwtSecret=<your-secret>
```

## Configuration Reference

| File | Description |
|------|-------------|
| `configs/routing-rules.yaml` | Region definitions, country lists, provider assignments |
| `configs/providers.yaml` | Provider URLs, auth, timeouts, circuit breaker settings |
| `configs/compliance-rules.yaml` | Global and per-jurisdiction compliance requirements |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_PORT` | `8080` | HTTP listen port |
| `JWT_SECRET` | `""` | JWT signing secret |
| `GEOIP_DB_PATH` | `/data/GeoLite2-Country.mmdb` | MaxMind database path |
| `CONFIG_DIR` | `configs` | Configuration directory |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness probe — always returns 200 |
| `GET` | `/readyz` | Readiness probe — checks provider circuit breakers |
| `ANY` | `/api/v1/*` | Proxy route — full middleware chain applied |

## Middleware Pipeline

```
RequestID → AuditLog → RateLimiter → Auth → RequestValidator →
GeoRouting → ComplianceCheck → ProxyRouter
```

## Supported Regions

| Region | Countries | License | Provider |
|--------|-----------|---------|----------|
| EU | DE, FR, ES, IT, NL, SE, FI, DK, PT, AT | MGA | Evolution Gaming / Playtech |
| UK | GB | UKGC | Pragmatic Play |
| LATAM | BR, MX, CO, AR, CL, PE | LOCAL | Playtech / Evolution Gaming |
| APAC | PH, JP, KR | PAGCOR | Asia Gaming |
| BLOCKED | US, AU, SG, HK, CN, IR, KP | — | Blocked |

## Development Setup

```bash
# Install dependencies
go mod download

# Build
go build ./...

# Run tests
go test ./...

# Run locally (requires GeoIP database and configs)
CONFIG_DIR=configs go run ./cmd/gateway
```

## Architecture Details

See [docs/architecture.md](docs/architecture.md) for a detailed architecture document.
