.PHONY: build run test test-cover lint swagger swagger-check docker tidy check \
        perf-up perf-smoke perf-test perf-down

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

# --- Performance testing (perf/) ---------------------------------------
# API_PORT is the host port the API is published on (18080 so it never clashes with `make run`).
PERF_COMPOSE := API_PORT=$(or $(API_PORT),18080) docker compose -f perf/docker-compose.yml

perf-up:     ## build the API image and start api + prometheus + grafana
	$(PERF_COMPOSE) up -d --build
	@echo "Grafana: http://localhost:3000   API: http://localhost:$(or $(API_PORT),18080)"

perf-smoke:  ## 1 VU for 30s across every endpoint; verifies the stack is wired
	$(PERF_COMPOSE) run --rm k6 run /scripts/smoke.js

perf-test:   ## ramp 0 -> 100 VUs over ~3.5 min; watch Grafana while it runs
	$(PERF_COMPOSE) run --rm k6 run /scripts/load.js

perf-down:   ## stop and remove the perf stack
	$(PERF_COMPOSE) down -v
