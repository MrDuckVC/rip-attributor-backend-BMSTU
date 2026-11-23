.DEFAULT_GOAL := lint

GO ?= go
DOCKER ?= docker
COMPOSE ?= $(DOCKER) compose
IMAGE ?= attributor:local
LINT_VERSION := v2.15.0
LINT_DIR := $(CURDIR)/bin/golangci-lint/$(LINT_VERSION)
LINT_BIN := $(LINT_DIR)/golangci-lint

.PHONY: lint lint-fix run docker-up docker-down docker-build migrate

$(LINT_BIN):
	curl -fsSL https://golangci-lint.run/install.sh | sh -s -- -b "$(LINT_DIR)" $(LINT_VERSION)

lint: $(LINT_BIN)
	"$(LINT_BIN)" run ./...

lint-fix: $(LINT_BIN)
	"$(LINT_BIN)" run --fix ./...

run:
	$(GO) run ./cmd/attributor

docker-up:
	$(COMPOSE) up -d --wait --wait-timeout 120

docker-down:
	$(COMPOSE) down

docker-build:
	$(DOCKER) build --tag "$(IMAGE)" .

migrate:
	@if [ -d cmd/migrate ]; then \
		$(GO) run ./cmd/migrate; \
	else \
		echo "В этой лабораторной нет миграций базы данных."; \
	fi
