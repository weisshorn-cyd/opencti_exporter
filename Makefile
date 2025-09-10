SHELL = /bin/sh

ENV ?= ./docker-compose.env

TARGETOS ?= linux
TARGETARCH ?= amd64

include $(ENV)

OPENCTI_TOKEN ?= $(OPENCTI_ADMIN_TOKEN)
OPENCTI_URL ?= $(OPENCTI_BASE_URL)

COMPOSE_FILE := ./docker-compose.yaml
COMPOSE_ENV_FILE := ./docker-compose.env

.PHONY: all build package fmt lint test vulncheck start-setup stop-setup clean

all: fmt lint

build:
	go build .

package:
	goreleaser release --snapshot --clean

fmt:
	gofumpt -l -w .

lint: clean
	golangci-lint run -c .golangci.yaml --fix ./...

vulncheck:
	govulncheck -test ./...

test:
	export OPENCTI_URL=$(OPENCTI_URL) && \
	export OPENCTI_TOKEN=$(OPENCTI_TOKEN) && \
	go test -failfast -race ./... -timeout 60s

start-setup:
	sudo docker compose --file $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) up -d

stop-setup:
	sudo docker compose --file $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) down

clean:
	go clean -cache -testcache -modcache
	rm -f opencti_exporter
	rm -rf dist
