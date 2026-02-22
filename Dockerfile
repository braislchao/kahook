ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o /kahook ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -g 1000 kahook && \
    useradd -u 1000 -g kahook -s /bin/sh -M kahook

WORKDIR /app

COPY --from=builder /kahook .
# Do NOT bake a config.yaml into the image — it could contain credentials.
# Supply configuration at runtime via:
#   - Environment variables (KAFKA_BROKERS, AUTH_TYPE, etc.)
#   - A mounted secret/configmap: -v /path/to/config.yaml:/app/config.yaml:ro

RUN chown -R kahook:kahook /app

USER kahook

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./kahook"]
