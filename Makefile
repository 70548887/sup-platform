.PHONY: build run test migrate

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/migrate ./cmd/migrate

run:
	go run ./cmd/api

worker:
	go run ./cmd/worker

migrate:
	go run ./cmd/migrate

test:
	go test ./...

lint:
	go vet ./...
