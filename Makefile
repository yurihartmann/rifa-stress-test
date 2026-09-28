.PHONY: up down logs ps test-api

COMPOSE := docker compose -f deploy/docker-compose.yml

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down --remove-orphans

logs:
	$(COMPOSE) logs -f --tail=200

ps:
	$(COMPOSE) ps

test-api:
	@if [ -f apps/api/go.mod ]; then \
		(cd apps/api && go test ./...); \
	else \
		echo "apps/api/go.mod ausente; teste ignorado"; \
	fi
