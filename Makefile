.PHONY: dev dev-all test test-sdk test-server lint lint-sdk lint-server build clean migrate test-e2e test-load

# Development
dev:
	docker compose up -d clickhouse postgres

dev-all:
	docker compose up -d

# Testing
test: test-sdk test-server

test-sdk:
	cd sdk-py && uv run pytest

test-server:
	cd server && go test ./...

test-e2e:
	python3 tests/e2e_test.py

test-load:
	python3 tests/load_test.py

# Linting
lint: lint-sdk lint-server

lint-sdk:
	cd sdk-py && uv run ruff check .
	cd sdk-py && uv run mypy src/

lint-server:
	cd server && go vet ./...

# Build
build:
	docker build -t axonize-server ./server

# Database
migrate:
	./migrations/migrate.sh

# Cleanup
clean:
	docker compose down -v
	rm -rf sdk-py/.venv sdk-py/dist
