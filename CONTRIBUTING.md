# Contributing

## Setup

Requires Go 1.22+, Docker, and Docker Compose.

```bash
git clone https://github.com/YOUR_USERNAME/kahook.git
cd kahook
make mod
make compose-up
```

## Workflow

```bash
make test       # Run tests
make lint       # Run linters
make fmt        # Format code
make build      # Build binary
```

Branch from `main`, ensure tests and linting pass, then open a PR.

Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`).

## Project Structure

```
cmd/server/          Entry point
internal/auth/       Authentication
internal/config/     Configuration
internal/kafka/      Kafka producer
internal/server/     HTTP server and handlers
deploy/              Kubernetes and Helm
```

## License

Contributions are licensed under MIT.
