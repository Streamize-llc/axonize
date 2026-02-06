# Contributing to Axonize

## Development Setup

### Prerequisites

- Python 3.10+ (native, not wasm32)
- Go 1.23+ (for server development)
- Docker and Docker Compose
- [uv](https://docs.astral.sh/uv/) (Python package manager)
- Node.js 22+ (for dashboard development)

### Clone and Setup

```bash
git clone https://github.com/your-org/axonize.git
cd axonize

# Start dev databases
make dev

# Apply migrations
make migrate
```

### Python SDK

```bash
cd sdk-py

# Create venv with native Python (important on macOS)
uv sync --python /opt/homebrew/bin/python3.13

# Run tests
uv run pytest -v

# Linting
uv run ruff check .
uv run mypy src/
```

### Go Server

```bash
cd server
go test ./...
go vet ./...
```

### Dashboard

```bash
cd dashboard
npm install
npm run dev      # Dev server on :3000
npm run build    # Production build
```

## Code Standards

### Python SDK

- **Formatter/Linter**: ruff (configured in `pyproject.toml`)
- **Type Checking**: mypy strict mode
- **Testing**: pytest
- **Target**: Python 3.10+
- Use `from __future__ import annotations` in all files
- All public APIs must have docstrings
- Inference thread overhead target: < 1us

### Go Server

- Standard `go vet` and `go test`
- Use structured logging (`log/slog`)
- Context-first function signatures
- Errors should be wrapped with `fmt.Errorf("context: %w", err)`

### Dashboard

- TypeScript strict mode
- ESLint (configured in `eslint.config.js`)
- Tailwind CSS v4 for styling
- React Query for data fetching

## Testing

```bash
# All tests
make test

# SDK only
make test-sdk

# Server only
make test-server

# E2E (requires full stack running)
make dev-all && make migrate && make test-e2e
```

## Project Structure

```
axonize/
├── sdk-py/          Python SDK
│   ├── src/axonize/ Source code
│   └── tests/       Unit tests
├── server/          Go server
│   ├── cmd/         Entry points
│   └── internal/    Internal packages
├── dashboard/       React dashboard
│   └── src/         Source code
├── migrations/      DB schemas
├── examples/        Integration examples
├── tests/           E2E tests
└── docs/            Documentation
```

## Making Changes

1. Create a branch from `main`
2. Make your changes
3. Ensure all tests pass: `make test && make lint`
4. Submit a pull request

## Commit Messages

Use conventional commit format:

```
feat: add llm_span API for streaming token tracking
fix: resolve GPU label lookup for MIG instances
docs: add vLLM integration example
test: add TTFT calculation edge case tests
```
