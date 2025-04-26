# Go parameters
# GOBASE := $(shell pwd) # Убрано, т.к. вызывает проблемы в Windows
# GOFILES := $(shell find . -name '*.go' -type f -not -path "./vendor/*") # Убрано
TARGET := retro_template

.PHONY: all build run clean lint help infra-up infra-down docker-build

all: build

build: 
	@go build -o $(TARGET) ./cmd/app/main.go

run:
	@go run ./cmd/app/main.go --config config/config.yml --wallets local/data/private_keys.txt

lint:
	@golangci-lint run ./...

COMPOSE_FILE := local/docker-compose.yml
COMPOSE := docker-compose -f $(COMPOSE_FILE) -p "retro"

docker-build: 
	@$(COMPOSE) build app

infra-up: 
	@$(COMPOSE) up -d

infra-down: 
	@$(COMPOSE) down
