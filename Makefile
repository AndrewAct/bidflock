.PHONY: all build test lint proto-gen docker-up docker-down infra-up infra-down seed simulate bench loadtest clean help

# ─── Config ──────────────────────────────────────────────────────────────────
QPS       ?= 1000
DURATION  ?= 60s
PATTERN   ?= steady
BID_URL   ?= http://localhost:8081/bid

SERVICES = bidding campaign budget scoring consumer

# ─── Build ───────────────────────────────────────────────────────────────────
all: build

build:
	@echo "Building all services..."
	@for svc in $(SERVICES); do \
		echo "  -> $$svc"; \
		go build -o bin/$$svc ./cmd/$$svc; \
	done
	@go build -o bin/simulator ./cmd/simulator
	@echo "Done. Binaries in ./bin/"

build/%:
	go build -o bin/$* ./cmd/$*

# ─── Code quality ─────────────────────────────────────────────────────────────
test:
	go test -race -count=1 ./...

test-verbose:
	go test -race -v -count=1 ./...

lint:
	@command -v golangci-lint &>/dev/null || { echo "Install golangci-lint: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

vet:
	go vet ./...

# ─── Proto generation ─────────────────────────────────────────────────────────
# Requires: protoc + protoc-gen-go + protoc-gen-go-grpc
# Install: https://grpc.io/docs/languages/go/quickstart/
proto-gen:
	@command -v protoc &>/dev/null || { echo "protoc not found. See https://grpc.io/docs/protoc-installation/"; exit 1; }
	@mkdir -p gen/go
	protoc \
		--go_out=gen/go --go_opt=paths=source_relative \
		--go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
		-I proto \
		proto/common.proto proto/scoring.proto proto/budget.proto proto/bidding.proto
	@echo "Proto generation complete. Files in gen/go/"

deps-proto:
	@echo "Installing protoc Go plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# ─── Docker ───────────────────────────────────────────────────────────────────
infra-up:
	@echo "Starting infrastructure (Redis, MongoDB, Redpanda, ClickHouse, Prometheus, Grafana)..."
	docker compose -f deployments/docker-compose.infra.yml up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@echo "Infrastructure ready."
	@echo "  Grafana:    http://localhost:3000 (admin/admin)"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Mongo:      mongodb://localhost:27017"
	@echo "  Redis:      localhost:6379"
	@echo "  Redpanda:   localhost:19092"

infra-down:
	docker compose -f deployments/docker-compose.infra.yml down

docker-up:
	@echo "Starting full stack..."
	docker compose -f deployments/docker-compose.yml build
	docker compose -f deployments/docker-compose.yml up -d
	@echo "Full stack running. Gateway at http://localhost:8080"

docker-down:
	docker compose -f deployments/docker-compose.yml down

docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f $(filter-out $@,$(MAKECMDGOALS))

docker-ps:
	docker compose -f deployments/docker-compose.yml ps

# ─── Run services locally ─────────────────────────────────────────────────────
run-bidding: build/bidding
	./bin/bidding

run-campaign: build/campaign
	./bin/campaign

run-budget: build/budget
	./bin/budget

run-scoring: build/scoring
	./bin/scoring

run-consumer: build/consumer
	./bin/consumer

# ─── Data & simulation ────────────────────────────────────────────────────────
seed:
	@echo "Seeding mock data (50 advertisers, 200 campaigns, 500 ads)..."
	go run scripts/seed.go
	@echo "Seed complete."

simulate:
	@echo "Starting traffic simulation: QPS=$(QPS) DURATION=$(DURATION) PATTERN=$(PATTERN)"
	go run cmd/simulator/main.go \
		--qps=$(QPS) \
		--duration=$(DURATION) \
		--pattern=$(PATTERN) \
		--bid-url=$(BID_URL)

bench:
	@echo "Running benchmark suite..."
	@mkdir -p benchmark-results
	bash scripts/benchmark.sh

loadtest:
	@echo "Running k6 extreme stress test..."
	bash scripts/loadtest.sh

# ─── Module management ────────────────────────────────────────────────────────
tidy:
	go mod tidy

download:
	go mod download

# ─── Cleanup ──────────────────────────────────────────────────────────────────
clean:
	rm -rf bin/ benchmark-results/

# ─── Help ─────────────────────────────────────────────────────────────────────
help:
	@echo "bidflock — RTB Ad Auction System"
	@echo ""
	@echo "Quick start:"
	@echo "  make infra-up          Start Redis, Mongo, Redpanda, ClickHouse"
	@echo "  make build             Build all service binaries"
	@echo "  make run-campaign      Start campaign service (port 8082)"
	@echo "  make run-budget        Start budget service gRPC (port 8083)"
	@echo "  make run-scoring       Start scoring service gRPC (port 8084)"
	@echo "  make run-bidding       Start bidding service (port 8081)"
	@echo "  make seed              Populate test data"
	@echo "  make simulate          Run traffic simulator"
	@echo ""
	@echo "Testing:"
	@echo "  make test              Run unit tests"
	@echo "  make bench             Run benchmark suite"
	@echo "  make loadtest          k6 extreme stress test"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up         Full stack via Docker Compose"
	@echo "  make docker-down       Tear down full stack"
	@echo ""
	@echo "Options:"
	@echo "  QPS=5000 DURATION=60s PATTERN=spike make simulate"
