ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache git gcc musl-dev pkgconf

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s -linkmode external -extldflags '-static'" -o /kahook ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 kahook && \
    adduser -u 1000 -G kahook -s /bin/sh -D kahook

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
