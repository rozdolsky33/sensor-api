.PHONY: build run test test-cover lint swagger swagger-check docker tidy check

BINARY := bin/server

build:
	go build -o $(BINARY) ./cmd/server

run:
	go run ./cmd/server

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -count=1 -coverprofile=coverage.out -coverpkg=./internal/... ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

swagger:
	go tool swag init -g cmd/server/main.go -o docs

swagger-check: swagger
	git diff --exit-code -- docs/docs.go docs/swagger.json docs/swagger.yaml

docker:
	docker build -t sensor-api .

tidy:
	go mod tidy

check: tidy lint test swagger-check
