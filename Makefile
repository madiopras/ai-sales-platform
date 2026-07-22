COMPOSE_DEV := docker compose --env-file .env.development -f infra/compose/docker-compose.base.yml -f infra/compose/docker-compose.dev.yml
COMPOSE_PROD := docker compose --env-file .env.production -f infra/compose/docker-compose.base.yml -f infra/compose/docker-compose.prod.yml

AI_DIR := apps/ai
AI_VENV := $(AI_DIR)/.venv
AI_PY := $(AI_VENV)/bin/python

.PHONY: dev dev-down dev-restart dev-logs dev-ps dev-config prod prod-down prod-logs prod-ps prod-config api-run api-test api-vet api-create-admin migrate-up migrate-down migrate-create clean ai-setup ai-run ai-test ai-lint ai-clean

dev:
	$(COMPOSE_DEV) up -d

dev-down:
	$(COMPOSE_DEV) down

dev-restart:
	$(COMPOSE_DEV) down
	$(COMPOSE_DEV) up -d

dev-logs:
	$(COMPOSE_DEV) logs -f

dev-ps:
	$(COMPOSE_DEV) ps

dev-config:
	$(COMPOSE_DEV) config --quiet

prod:
	$(COMPOSE_PROD) up -d

prod-down:
	$(COMPOSE_PROD) down

prod-logs:
	$(COMPOSE_PROD) logs -f

prod-ps:
	$(COMPOSE_PROD) ps

prod-config:
	$(COMPOSE_PROD) config --quiet

api-run:
	cd apps/api && if [ -f .env ]; then set -a; . ./.env; set +a; fi; go run ./cmd/api

api-test:
	cd apps/api && go test ./...

api-vet:
	cd apps/api && go vet ./...

api-create-admin:
	cd apps/api && if [ -f .env ]; then set -a; . ./.env; set +a; fi; go run ./cmd/admin

clean:
	$(COMPOSE_DEV) down -v

# --- AI service (apps/ai, FastAPI) ---

# Create the virtualenv and install runtime + dev dependencies.
ai-setup:
	cd $(AI_DIR) && python3 -m venv .venv && .venv/bin/pip install --upgrade pip && .venv/bin/pip install -e ".[dev]"
	@[ -f $(AI_DIR)/.env ] || cp $(AI_DIR)/.env.example $(AI_DIR)/.env

# Run the service with autoreload (reads apps/ai/.env). Requires `make ai-setup` first.
ai-run:
	cd $(AI_DIR) && .venv/bin/uvicorn app.main:app --reload --host 0.0.0.0 --port 8090

ai-test:
	cd $(AI_DIR) && .venv/bin/python -m pytest

ai-lint:
	cd $(AI_DIR) && .venv/bin/ruff check app tests

ai-clean:
	rm -rf $(AI_VENV) $(AI_DIR)/.pytest_cache $(AI_DIR)/.ruff_cache
