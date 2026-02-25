# Kahook

A lightweight webhook-to-Kafka bridge. `POST /{topic}` publishes the request body to that Kafka topic.

## Quick Start

```bash
# Start Kahook + Kafka + Kafka UI
make compose-up

# Send a webhook (no auth by default)
curl -X POST http://localhost:8080/my-topic \
  -H "Content-Type: application/json" \
  -d '{"event": "test"}'
```

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/{topic}` | POST | Publish to Kafka topic |
| `/health` | GET | Health check |
| `/ready` | GET | Readiness (Kafka connectivity) |
| `/metrics` | GET | Server metrics (auth required if configured) |

## Authentication

Kahook auto-detects the scheme from the `Authorization` header. Configure users and/or tokens via `config.yaml` or environment variables:

```yaml
auth:
  users:
    - username: admin
      password: secret
  tokens:
    - my-bearer-token
```

Or via environment variables (useful for Kubernetes Secrets):

```bash
AUTH_TOKENS=my-bearer-token
AUTH_BASIC_USERS=admin:secret,reader:pass123
```

If both are configured, clients can use either. If neither is configured, all requests are allowed.

## Configuration

Via `config.yaml` or environment variables:

```yaml
server:
  port: 8080
  read_timeout: 10
  write_timeout: 10
  idle_timeout: 60

kafka:
  brokers:
    - localhost:9092
  acks: all
  retries: 3
  compression_type: snappy
```

### Confluent Cloud

Via `config.yaml`:

```yaml
kafka:
  brokers:
    - pkc-xxxxx.us-east-1.aws.confluent.cloud:9092
  sasl_username: your-api-key
  sasl_password: your-api-secret
  sasl_mechanism: PLAIN
  security_protocol: SASL_SSL
```

Or via environment variables:

```bash
KAFKA_BROKERS=pkc-xxxxx.us-east-1.aws.confluent.cloud:9092
KAFKA_SASL_USERNAME=your-api-key
KAFKA_SASL_PASSWORD=your-api-secret
KAFKA_SASL_MECHANISM=PLAIN
KAFKA_SECURITY_PROTOCOL=SASL_SSL
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SERVER_PORT` | HTTP port |
| `AUTH_TYPE` | Auth method: `none`, `basic`, or `bearer` |
| `AUTH_TOKENS` | Comma-separated bearer tokens |
| `AUTH_BASIC_USERS` | Comma-separated `user:pass` pairs (e.g. `admin:secret,reader:pass`) |
| `KAFKA_BROKERS` | Comma-separated brokers |
| `KAFKA_SASL_USERNAME` | SASL username |
| `KAFKA_SASL_PASSWORD` | SASL password |
| `KAFKA_SASL_MECHANISM` | SASL mechanism (default: `PLAIN`) |
| `KAFKA_SECURITY_PROTOCOL` | Security protocol (default: `PLAINTEXT`) |

## Webhook Headers

Request headers are forwarded as Kafka message headers, except standard HTTP headers (`Authorization`, `Content-Type`, `Host`, etc.).

Set `X-Webhook-Key` to control the Kafka message key.

## Deployment

```bash
# Docker
make docker-build
docker run -p 8080:8080 -e KAFKA_BROKERS=kafka:9092 kahook:latest

# Kubernetes
kubectl apply -f deploy/kubernetes/kahook.yaml

# Helm
helm install kahook ./deploy/helm/kahook
```

## Development

```bash
make mod      # Install dependencies
make run      # Run locally
make test     # Run tests
make lint     # Run linters
make build    # Build binary
```
