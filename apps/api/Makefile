.PHONY: test build docs

SWAG_DIR := ./cmd/api,./internal/interface/http,./internal/domain/dto,./internal/domain/entity

test:
	go test ./...

build:
	go build ./...

docs:
	swag fmt --dir $(SWAG_DIR) && swag init --dir $(SWAG_DIR) --parseDependency --parseInternal --output api --packageName api
