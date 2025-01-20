SHELL = /bin/sh

ENV ?= ./docker-compose.env

TARGETOS ?= linux
TARGETARCH ?= amd64

include $(ENV)

OPENCTI_TOKEN ?= $(OPENCTI_ADMIN_TOKEN)
OPENCTI_URL ?= $(OPENCTI_BASE_URL)

COMPOSE_FILE := ./docker-compose.yaml
COMPOSE_ENV_FILE := ./docker-compose.env

all: fmt lint

.PHONY: build
build:
	go build .

.PHONY: package
package:
	goreleaser release --snapshot --clean

.PHONY: fmt
fmt:
	gofumpt -l -w .
	wsl --fix ./...

.PHONY: lint
lint:
	golangci-lint run --fix

.PHONY: test
test:
	export OPENCTI_URL=$(OPENCTI_URL) && \
	export OPENCTI_TOKEN=$(OPENCTI_TOKEN) && \
	go test -failfast -race ./... -timeout 60s

.PHONY: start-setup
start-setup:
	sudo docker compose --file $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) up -d

.PHONY: stop-setup
stop-setup:
	sudo docker compose --file $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) down

.PHONY: clean
clean:
	go clean -cache -testcache -modcache
	rm -f opencti_exporter
	rm -rf dist
