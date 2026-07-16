COMPOSE_DEV := docker compose --env-file .env.development -f infra/compose/docker-compose.base.yml -f infra/compose/docker-compose.dev.yml
COMPOSE_PROD := docker compose --env-file .env.production -f infra/compose/docker-compose.base.yml -f infra/compose/docker-compose.prod.yml

.PHONY: dev dev-down dev-restart dev-logs dev-ps dev-config prod prod-down prod-logs prod-ps prod-config api-run api-test api-vet clean

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

clean:
	$(COMPOSE_DEV) down -v
