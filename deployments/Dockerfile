FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/gateway ./cmd/gateway

FROM alpine:3.19

RUN addgroup -S gateway && adduser -S -G gateway gateway

VOLUME /data
WORKDIR /app

COPY --from=builder /bin/gateway /app/gateway
COPY configs/ /app/configs/

USER gateway

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/gateway"]
