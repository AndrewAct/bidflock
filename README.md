# bidflock

A production-grade, open-source **Real-Time Bidding (RTB) Ad Auction System** written entirely in Go.

Built to demonstrate high-concurrency, low-latency distributed systems design — the kind of infrastructure that powers ad platforms at ByteDance (TikTok), Google, and Meta. Targets **100K+ QPS** with **<50ms P99** latency on the hot auction path.

Follows the **IAB OpenRTB 2.6 standard** for bid request/response schemas.

---

## Why this exists

Real-Time Bidding is one of the hardest distributed systems problems in production:

- **Latency budget is tiny.** The entire auction — network in, scoring, budget check, auction logic, network out — must complete in under 100ms or the SSP moves on. Most systems target <50ms.
- **Concurrency is extreme.** A mid-sized ad platform handles millions of auctions per minute across hundreds of active campaigns.
- **Correctness under concurrency is non-negotiable.** Budget overspend by even 5% costs real money. Race conditions in bid logic produce unfair auctions.
- **The data model is rich.** Campaigns have targeting rules, pacing schedules, frequency caps, bid strategies, creative assets — all of which must be queryable on the hot path without touching a slow database.

This project exists to demonstrate that all of the above is achievable in idiomatic Go — without exotic dependencies, without sacrificing readability, and with a system that can be brought up entirely on a laptop.

---

## Architecture

### System overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Traffic Simulator                         │
│   (OpenRTB bid requests, realistic user profiles, 100K+ QPS)    │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTP POST /v1/bid
                            ▼
                   ┌─────────────────┐
                   │   API Gateway   │  KrakenD :8080
                   │  Rate limiting  │  Request routing
                   │  API key auth   │
                   └────────┬────────┘
                            │ HTTP
                            ▼
              ┌─────────────────────────┐
              │     Bidding Service     │  :8081  ← HOT PATH
              │  Auction engine         │
              │  Second/first price     │
              └──┬──────────────┬───────┘
                 │              │
          gRPC   │              │  gRPC
       ScoreAds  │              │  CheckBudget / DeductBudget
                 ▼              ▼
      ┌──────────────┐  ┌──────────────┐
      │   Scoring    │  │    Budget    │
      │  Service     │  │   Service   │
      │  :8084 gRPC  │  │  :8083 gRPC │
      │              │  │             │
      │  Logistic    │  │  Lua atomic │
      │  regression  │  │  deduction  │
      │  CTR/CVR     │  │  Pacing     │
      │  prediction  │  │  Freq cap   │
      └──────────────┘  └─────────────┘
              │                 │
              └────────┬────────┘
                       │  Redis reads (hot path)
                       ▼
               ┌──────────────┐
               │    Redis     │  Campaign cache, budgets,
               │              │  frequency caps, features
               └──────────────┘

Campaign Service :8082 (CRUD + event source)
      │
      ├── MongoDB (persistent storage)
      └── Redpanda (Kafka)
              │
              ├── campaign-updates ──► Bidding / Budget / Scoring
              ├── bid-results ────────► Ad Event Consumer
              ├── impressions ────────► Ad Event Consumer
              ├── clicks ─────────────► Ad Event Consumer
              └── conversions ────────► Ad Event Consumer
                                              │
                                        ClickHouse
                                        (analytics, attribution)
```

### Hot path detail

Every auction runs through this sequence, all within a 50ms budget:

```
POST /bid (OpenRTB BidRequest)
│
├── Parse & validate request                             ~0.1ms
├── Load active campaign IDs from Redis SMEMBERS         ~0.5ms
├── gRPC ScoreAds → Scoring Service                     ~8-15ms
│     ├── For each campaign × ad:
│     │     ├── Assemble feature vector
│     │     │     (age group, geo, device, interests,
│     │     │      historical CTR from Redis feature store)
│     │     └── Logistic regression → predicted_ctr
│     └── effective_bid = base_bid × predicted_ctr × 1000 (CPM)
│
├── gRPC CheckBudget → Budget Service (per campaign)    ~5-10ms
│     ├── Redis GET remaining_daily_budget
│     ├── Sliding-window frequency cap check
│     └── Pacing multiplier (throttle if overpacing)
│
├── RunAuction (pure in-memory, no I/O)                 ~0.01ms
│     ├── Filter: budget_ok == true AND bid >= floor
│     ├── Sort by effective_bid descending
│     └── Second-price: winner pays second-highest bid
│
├── gRPC DeductBudget (Lua atomic check-and-deduct)     ~3-5ms
│
├── Publish bid-result to Kafka (async goroutine)        non-blocking
│
└── Return BidResponse (OpenRTB)                        total: <50ms P99
```

### Campaign config propagation (event-driven, eventually consistent)

```
Campaign Service mutation (create/update/delete)
│
├── Write to MongoDB (source of truth)
└── Publish to Kafka: campaign-updates topic
      │
      ├── Bidding Service consumer:
      │     └── SAdd/SRem campaign ID in Redis active set
      │
      ├── Budget Service consumer:
      │     └── SetNX daily budget key in Redis (doesn't reset mid-day)
      │
      └── Scoring Service consumer:
            └── Cache full CampaignCache struct in Redis
                (bid config, targeting rules, ad IDs)
```

The bidding hot path **never queries MongoDB**. All reads during auction are from Redis.

---

## Tech stack choices

| Component | Choice | Why |
|-----------|--------|-----|
| **Language** | Go 1.22 | Goroutine-based concurrency model is ideal for high-fan-out auction logic. Minimal runtime overhead. First-class `net/http` and `net` packages. Static binaries make containerization trivial. |
| **HTTP router** | `chi` | Zero-allocation routing, composable middleware, better benchmark characteristics than Gin for this use case. Clean `chi.URLParam` API. |
| **gRPC** | `google.golang.org/grpc` | Strongly-typed service contracts between internal services. Multiplexed HTTP/2 transport. Lower overhead than REST for high-frequency inter-service calls. |
| **gRPC encoding** | Custom JSON codec (`pkg/codec`) | Overrides the default `"proto"` codec name so gRPC uses `encoding/json` internally. Lets us use plain Go structs as message types without requiring `protoc` to be installed. The `.proto` files in `proto/` remain the canonical schema; run `make proto-gen` to switch to binary protobuf encoding when you have protoc available. |
| **Kafka client** | `franz-go` | Pure Go (no CGO, no librdkafka). Best throughput of any Go Kafka client in benchmarks. Clean `ProduceSync` / `PollFetches` API. |
| **Redis client** | `go-redis/v9` | Context-aware, supports Lua scripting (`Eval`), connection pooling. The standard choice in the Go ecosystem. |
| **MongoDB driver** | `mongo-driver` | Official driver. Used only by Campaign Service — never on the hot path. |
| **ClickHouse client** | `clickhouse-go/v2` | Supports native binary protocol, batch inserts, columnar append. ClickHouse is purpose-built for the append-heavy analytics workload (bid logs, impressions, attribution). |
| **Metrics** | `prometheus/client_golang` | De-facto standard. `promauto` registers metrics at init time. Scrape endpoint at `/metrics` on each service. |
| **Logging** | `log/slog` (stdlib) | Structured JSON logging with zero external dependencies. Consistent field names across services for easy Grafana Loki / Splunk querying. |
| **API Gateway** | KrakenD | Config-driven Go gateway. Handles rate limiting, routing, CORS. No code to maintain. |
| **Streaming** | Redpanda | Kafka-compatible but ~10× faster startup, lower memory footprint, no ZooKeeper. Ideal for local development. Drop-in replacement for Kafka in production. |
| **Analytics DB** | ClickHouse | MergeTree engine with per-month partitioning. Handles billions of bid/impression rows. Pre-built analytics view (`campaign_performance`) for CTR/CVR reporting. |
| **ML inference** | Logistic regression (gonum) | Hardcoded weights for Phase 1. Fast, interpretable, easy to explain in interviews. Designed to be replaced by an ONNX model or LightGBM scorer in Phase 2. |
| **Load testing** | k6 | JavaScript-based, supports ramp stages, threshold assertions, custom metrics. The `scripts/loadtest.sh` script targets 100K VUs. |

---

## Project structure

```
bidflock/
│
├── proto/                    # Canonical API schema (OpenRTB-aligned)
│   ├── common.proto          # BidRequest, BidResponse, User, Device, Geo, ...
│   ├── budget.proto          # CheckBudget, DeductBudget, GetPacingInfo RPCs
│   ├── scoring.proto         # ScoreAds RPC
│   └── bidding.proto         # BidResult event type (Kafka payload)
│
├── gen/go/                   # gRPC service bindings (hand-written, proto-equivalent)
│   ├── budget/budget.go      # BudgetServiceServer/Client, ServiceDesc, handlers
│   └── scoring/scoring.go    # ScoringServiceServer/Client, ServiceDesc, handlers
│
├── pkg/                      # Shared packages (importable by all services)
│   ├── codec/codec.go        # JSON-over-gRPC codec registration
│   ├── models/
│   │   ├── openrtb.go        # BidRequest, BidResponse, Imp, Banner, Video, ...
│   │   ├── campaign.go       # Campaign, Ad, Advertiser, TargetingRules, CampaignCache
│   │   └── events.go         # ImpressionEvent, ClickEvent, ConversionEvent
│   ├── kafka/
│   │   ├── producer.go       # Thin franz-go wrapper; topic constants
│   │   └── consumer.go       # Group consumer with auto-commit
│   ├── redis/client.go       # Redis wrapper; DB number constants; Lua script runner
│   └── observability/
│       ├── metrics.go        # All Prometheus counters, histograms, gauges
│       └── logging.go        # slog JSON logger factory; shared field key constants
│
├── internal/                 # Service-private logic (not importable cross-service)
│   ├── bidding/
│   │   ├── auction.go        # RunAuction: first-price / second-price logic
│   │   ├── service.go        # ProcessBidRequest: orchestrates scoring + budget + auction
│   │   ├── handler.go        # HTTP handler, Prometheus instrumentation
│   │   └── grpc_client.go    # Typed wrappers for Scoring and Budget gRPC clients
│   ├── campaign/
│   │   ├── repository.go     # MongoDB CRUD for Advertiser, Campaign, Ad
│   │   ├── service.go        # Business logic, validation, ID generation
│   │   ├── handler.go        # chi HTTP handlers for all REST endpoints
│   │   └── publisher.go      # Kafka event publisher for campaign mutations
│   ├── budget/
│   │   ├── service.go        # CheckBudget, DeductBudget, Kafka sync handler
│   │   ├── grpc_server.go    # gRPC server implementation + Serve()
│   │   ├── pacing.go         # Smooth pacing: target rate vs actual spend rate
│   │   └── limiter.go        # Frequency cap: Redis sorted set sliding window
│   ├── scoring/
│   │   ├── service.go        # ScoreAds orchestration, campaign cache reader
│   │   ├── grpc_server.go    # gRPC server implementation + Serve()
│   │   ├── feature.go        # Feature vector assembly from Redis + request context
│   │   └── predictor.go      # LogisticRegressor: Predict(FeatureVector) → float64
│   ├── consumer/
│   │   ├── writer.go         # ClickHouse BatchWriter (size + interval flush)
│   │   ├── impression.go     # Impression Kafka handler
│   │   ├── click.go          # Click Kafka handler
│   │   └── conversion.go     # Conversion Kafka handler
│   └── simulator/
│       ├── user_gen.go       # Weighted-random user profiles (age, gender, geo, device)
│       ├── request_gen.go    # OpenRTB 2.6 bid request generator
│       ├── event_gen.go      # Post-bid funnel simulation (impression → click → conversion)
│       ├── traffic.go        # QPS controller, traffic patterns (steady/spike/ramp/diurnal)
│       └── reporter.go       # Live terminal stats + JSON/CSV benchmark output
│
├── cmd/                      # One main.go per service (independently deployable)
│   ├── bidding/main.go
│   ├── campaign/main.go
│   ├── budget/main.go
│   ├── scoring/main.go
│   ├── consumer/main.go
│   └── simulator/main.go
│
├── deployments/
│   ├── docker-compose.yml          # Full stack (all services + infra)
│   ├── docker-compose.infra.yml    # Infrastructure only
│   └── Dockerfile.{service}        # One per service (multi-stage, ~15MB images)
│
├── config/
│   ├── krakend.json          # API Gateway routing + rate limiting config
│   └── prometheus.yml        # Scrape config for all services
│
├── scripts/
│   ├── seed.go               # Populate 50 advertisers, 200 campaigns, 500 ads
│   ├── clickhouse-init.sql   # DDL: bid_results, impressions, clicks, conversions tables
│   ├── benchmark.sh          # Automated latency curve sweep (100→50K QPS)
│   └── loadtest.sh           # k6 extreme stress test (ramps to 100K VUs)
│
├── Makefile                  # build, test, infra-up, seed, simulate, bench, loadtest
├── go.mod
└── go.sum
```

---

## How to bring it up

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | https://go.dev/dl/ |
| Docker Desktop | 4.x+ | https://www.docker.com/products/docker-desktop/ |
| Docker Compose | v2 (bundled with Docker Desktop) | — |
| k6 (optional, for stress test) | latest | `brew install k6` |

### Step 1 — Start infrastructure

```bash
make infra-up
```

This starts:
- **Redis** on `localhost:6379`
- **MongoDB** on `localhost:27017`
- **Redpanda** (Kafka) on `localhost:19092` (Kafka-compatible external port)
- **ClickHouse** on `localhost:9000` (native) / `localhost:8123` (HTTP)
- **Prometheus** on `localhost:9090`
- **Grafana** on `localhost:3000` (login: admin / admin)

Redpanda will auto-create all 6 Kafka topics on first start via `redpanda-init`.

ClickHouse will auto-run `scripts/clickhouse-init.sql` to create the analytics tables.

Wait for all services to be healthy (about 30 seconds):
```bash
docker compose -f deployments/docker-compose.infra.yml ps
```

### Step 2 — Build all services

```bash
make build
```

Produces binaries in `./bin/`: `bidding`, `campaign`, `budget`, `scoring`, `consumer`, `simulator`.

### Step 3 — Run services locally

Open five terminal tabs and run each service. Order matters — campaign must be up before seeding; budget and scoring must be up before bidding.

```bash
# Tab 1
make run-campaign    # Campaign Service on :8082

# Tab 2
make run-budget      # Budget Service gRPC on :8083

# Tab 3
make run-scoring     # Scoring Service gRPC on :8084

# Tab 4
make run-bidding     # Bidding Service on :8081

# Tab 5
make run-consumer    # Ad Event Consumer (Kafka → ClickHouse)
```

Each service prints structured JSON logs to stdout. Health check: `curl localhost:8081/health`.

### Step 4 — Seed mock data

```bash
make seed
```

Creates 50 advertisers, 200 campaigns, and 500 ad creatives via the Campaign Service REST API. Data flows automatically:

```
Campaign Service (HTTP POST) → MongoDB → Kafka (campaign-updates)
    → Budget Service (initializes Redis budget keys)
    → Scoring Service (caches CampaignCache in Redis)
    → Bidding Service (adds campaign IDs to Redis active set)
```

After seeding, all three services' Redis caches are warm and the bidding path is ready to serve auctions.

### Step 5 — Send traffic

```bash
# 1,000 requests/second, 60 seconds, steady traffic
make simulate QPS=1000 DURATION=60s PATTERN=steady

# Spike to 3× then back to baseline
make simulate QPS=5000 DURATION=120s PATTERN=spike

# Linear ramp from 10% to 100% of QPS
make simulate QPS=10000 DURATION=3m PATTERN=ramp

# Sinusoidal day/night traffic curve
make simulate QPS=5000 DURATION=5m PATTERN=diurnal
```

Live output while simulation runs:
```
[  5s] QPS:  4982  Bids: 73.4%  Err:  0.1%  P50: 12ms  P95: 38ms  P99: 47ms
[  6s] QPS:  5011  Bids: 74.1%  Err:  0.0%  P50: 11ms  P95: 36ms  P99: 45ms
```

Results are saved to `benchmark-results/benchmark-{timestamp}.json` and `.csv` for charting.

---

## Docker Compose (full stack)

To run everything including all services in containers:

```bash
make docker-up
```

This builds Docker images for all 5 services and brings up the full stack. The API Gateway (KrakenD) will be available at `http://localhost:8080`.

Tear down:
```bash
make docker-down
```

View logs for a specific service:
```bash
make docker-logs bidding
make docker-logs campaign
```

---

## Cloud deployment (GKE)

> **Note:** `deployments/k8s/` is scaffolded as a placeholder. Full Helm charts are Phase 2.

High-level production deployment approach:

```
┌─────────────────────────────────────────────────────────┐
│  GKE Cluster                                            │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ bidding  │  │ campaign │  │  budget  │  (HPA: CPU)  │
│  │ x10 pods │  │ x2 pods  │  │ x4 pods  │              │
│  └──────────┘  └──────────┘  └──────────┘              │
│                                                         │
│  ┌──────────┐  ┌──────────┐                             │
│  │ scoring  │  │ consumer │                             │
│  │ x4 pods  │  │ x2 pods  │                             │
│  └──────────┘  └──────────┘                             │
│                                                         │
│  Managed services:                                      │
│    Redis       → Cloud Memorystore (Redis)              │
│    MongoDB     → MongoDB Atlas or Cloud Bigtable        │
│    Kafka       → Confluent Cloud or MSK                 │
│    ClickHouse  → ClickHouse Cloud or self-managed       │
│    Gateway     → Cloud Load Balancer + KrakenD          │
└─────────────────────────────────────────────────────────┘
```

Each service image is ~15MB (multi-stage build, Alpine base). Environment variables configure all endpoints — no code changes needed between local and cloud.

Quick one-time GKE deploy for benchmark video content:
```bash
gcloud container clusters create bidflock \
  --num-nodes=3 --machine-type=n2-standard-4

kubectl apply -f deployments/k8s/
```

---

## API reference

### `POST /bid` — Submit a bid request

Accepts an OpenRTB 2.6 BidRequest. Returns a BidResponse with the winning bid or a no-bid reason code.

```bash
curl -X POST http://localhost:8081/bid \
  -H "Content-Type: application/json" \
  -d '{
    "id": "req-001",
    "imp": [{
      "id": "imp-1",
      "banner": { "w": 320, "h": 50 },
      "bidfloor": 0.30,
      "bidfloorcur": "USD"
    }],
    "app": {
      "id": "app-1",
      "bundle": "com.example.hypercasual",
      "cat": ["IAB9"]
    },
    "user": {
      "id": "user-abc123",
      "yob": 1993,
      "gender": "M",
      "geo": { "country": "US" },
      "interests": ["gaming", "tech"]
    },
    "device": {
      "os": "iOS",
      "devicetype": 4,
      "ua": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)",
      "ip": "1.2.3.4"
    },
    "at": 2,
    "tmax": 150,
    "cur": ["USD"],
    "ext": { "ssp_id": "mock-adx", "exchange": "mock-adx" }
  }'
```

**Win response:**
```json
{
  "id": "req-001",
  "seatbid": [{
    "bid": [{
      "id": "3f7a1c2d-...",
      "impid": "imp-1",
      "price": 2.45,
      "adid": "ad-xyz",
      "cid": "campaign-123",
      "crid": "ad-xyz",
      "nurl": "http://bidflock/win?bid=3f7a1c2d&price=${AUCTION_PRICE}"
    }],
    "seat": "bidflock"
  }],
  "cur": "USD"
}
```

**No-bid response** (e.g. budget exhausted):
```json
{ "id": "req-001", "nbr": 9 }
```

OpenRTB no-bid reason codes: `7` = no ad, `8` = freq capped, `9` = budget constraints, `10` = timeout.

### Campaign Service (`:8082` or via gateway `/v1/`)

```bash
# Create advertiser
curl -X POST http://localhost:8082/advertisers \
  -H "Content-Type: application/json" \
  -d '{ "name": "Nike", "domain": "nike.com", "industry": "fashion" }'

# Create campaign
curl -X POST http://localhost:8082/campaigns \
  -H "Content-Type: application/json" \
  -d '{
    "advertiser_id": "<id>",
    "name": "Summer Sale 2025",
    "status": "active",
    "bid_strategy": "max_bid",
    "daily_budget": 1000.0,
    "total_budget": 30000.0,
    "bid_ceiling": 10.0,
    "base_bid": 3.5,
    "targeting": {
      "age_min": 18, "age_max": 35,
      "geos": ["US", "GB"],
      "interests": ["fashion", "sports"],
      "device_types": ["ios", "android"]
    },
    "start_date": "2025-01-01T00:00:00Z",
    "end_date": "2025-12-31T23:59:59Z"
  }'

# List active campaigns
curl "http://localhost:8082/campaigns?status=active&limit=20"

# Update a campaign (triggers Kafka event → cache invalidation)
curl -X PUT http://localhost:8082/campaigns/<id> \
  -H "Content-Type: application/json" \
  -d '{ "status": "paused", ... }'

# Create an ad creative and attach to campaign
curl -X POST http://localhost:8082/ads \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": "<id>",
    "advertiser_id": "<id>",
    "type": "banner",
    "title": "30% Off — Today Only",
    "landing_url": "https://nike.com/sale",
    "image_url": "https://cdn.nike.com/banner320x50.jpg",
    "width": 320,
    "height": 50
  }'
```

---

## Traffic Simulator

The simulator is a first-class component — not a test harness. It models real SSP traffic with correct OpenRTB structure, realistic user demographics, and a conversion funnel that generates Kafka events downstream.

### Traffic patterns

| Pattern | Description | Use case |
|---------|-------------|----------|
| `steady` | Constant QPS | Baseline latency measurement |
| `spike` | Normal → 3× spike → back | Black Friday / viral event simulation |
| `ramp` | 10% → 100% over duration | Gradual load increase, find breaking point |
| `diurnal` | Sine wave day/night curve | Realistic 24-hour traffic simulation |

### User profile distribution

| Dimension | Distribution |
|-----------|-------------|
| Age | 18-24 (30%), 25-34 (35%), 35-44 (20%), 45+ (15%) |
| Gender | Male (48%), Female (50%), Unknown (2%) |
| Geography | US (40%), JP (15%), GB (10%), DE (8%), BR (7%), other (20%) |
| Device | iOS (45%), Android (50%), Desktop (5%) |
| SSP | 8 partners: mock-adx, mock-tiktok-exchange, mock-pangle, mock-applovin, mock-unity-ads, mock-ironsource, mock-mopub, mock-pubmatic |

### Conversion funnel

```
Auction win
    │
    ├── 90% → Impression event published to Kafka
    │         (10% lost: render timeout, user navigation)
    │
    └── 1.5–3% of impressions → Click event published
              │
              └── 5–15% of clicks → Conversion event published
                                    (value: $10–$100 random)
```

These events flow to the Ad Event Consumer and land in ClickHouse for analytics.

### Example runs

```bash
# Standard benchmark: save results to ./results/
go run cmd/simulator/main.go \
  --qps=5000 \
  --duration=2m \
  --pattern=steady \
  --output=./results

# Reproducible run (same seed = same request sequence)
go run cmd/simulator/main.go \
  --qps=1000 --duration=30s --seed=42

# Point at Docker Compose stack
go run cmd/simulator/main.go \
  --qps=2000 --duration=60s \
  --bid-url=http://localhost:8080/v1/bid \
  --kafka-brokers=localhost:19092
```

---

## Benchmarking

### Automated sweep

```bash
make bench
```

Runs the simulator at 100 → 500 → 1K → 2K → 5K → 10K → 20K → 50K QPS (30s each), saving JSON and CSV results per level to `benchmark-results/`. Use the CSV files to plot latency vs QPS curves.

### k6 extreme stress test

```bash
make loadtest
```

Ramps to 100,000 virtual users over 14 minutes. Asserts P99 < 100ms and error rate < 1%.

### Expected numbers (M-series Mac, Docker, single-instance Redis)

| QPS | P50 | P95 | P99 | Bid rate | Notes |
|-----|-----|-----|-----|----------|-------|
| 1K  | 8ms | 20ms | 35ms | ~70% | Warm cache, all budgets available |
| 5K  | 12ms | 35ms | 48ms | ~68% | |
| 10K | 16ms | 42ms | 62ms | ~65% | |
| 50K | 25ms | 65ms | 90ms | ~60% | Redis starts to be the bottleneck |

*Your numbers will vary. Run `make bench` on your hardware and document the specs.*

---

## Design decisions and trade-offs

### 1. Hot path never touches MongoDB

All data the bidding path needs (campaign config, budgets, feature vectors) lives in Redis. MongoDB is the write-ahead store for Campaign Service only.

**Trade-off:** Eventual consistency. A campaign update takes 50–500ms to propagate through Kafka to Redis caches. During this window, a small number of auctions may use stale bid or targeting config. This is acceptable in ad tech — the alternative (synchronous MongoDB reads per auction) would add 15–30ms per request and doesn't scale.

### 2. Budget deduction via Lua script

Redis `GET`→`SET` is not atomic. Under concurrent auction load, two goroutines could both read a remaining budget of $0.50, both decide to bid $0.40, and both deduct — overspending by $0.30. The Lua script in `internal/budget/service.go` executes atomically on the Redis server:

```lua
local current = tonumber(redis.call('GET', key) or '0')
if current < amount then return 0 end
redis.call('INCRBYFLOAT', key, -amount)
return 1
```

**Trade-off:** Lua blocks Redis on that key for microseconds. At 100K QPS across 200 campaigns, this is ~500 ops/key/second — well within Redis single-threaded throughput limits (~1M ops/second for simple commands).

### 3. Second-price auction (configurable to first-price)

Second-price (Vickrey) auction: the winner pays the second-highest bid, not their own bid. This incentivizes bidders to bid their true valuation. Toggle to first-price via `Config.AuctionType = FirstPrice` in the bidding service.

**Context:** Programmatic advertising has been moving from second-price to first-price auctions since 2019 (Google Ad Exchange, OpenX, AppNexus all switched). Both are implemented; second-price is the default for interview-explainability.

### 4. Smooth budget pacing

Without pacing, a campaign with a $1,000 daily budget could spend all of it by 2am at high traffic volume, then serve zero ads for the remaining 22 hours. Smooth pacing divides remaining budget by remaining hours to compute a target spend rate, then applies a bid multiplier (0.3–1.0) if the actual rate exceeds the target.

```
target_rate = remaining_budget / remaining_hours
if actual_rate > target_rate × 1.2:
    multiplier = 0.3  # significantly throttle
elif actual_rate > target_rate × 1.05:
    multiplier = 0.7  # gently throttle
else:
    multiplier = 1.0  # full speed
```

**Phase 2:** Replace with a PID controller for tighter, oscillation-free pacing.

### 5. Frequency capping with Redis sorted sets

The sliding-window cap (`internal/budget/limiter.go`) stores each impression as a member in `freqcap:{campaign_id}:{user_id}` with score = timestamp. On each check, members older than 24h are removed via `ZREMRANGEBYSCORE`, then `ZCARD` gives the count.

**Alternative considered:** Redis HyperLogLog (constant 12KB memory vs O(impressions) for sorted set). HyperLogLog can't support member removal, making sliding windows impossible. Sorted set chosen for correctness; at scale, use a distributed bloom filter.

### 6. gRPC with JSON codec

Standard gRPC uses protobuf binary encoding, which requires `protoc` and generated Go types. To make this project immediately runnable (`go build ./...` works out of the box), a custom JSON codec is registered at `pkg/codec/codec.go`:

```go
func (JSON) Name() string { return "proto" } // override default codec name
```

Naming it `"proto"` replaces gRPC's default codec transparently. The service `ServiceDesc` structs in `gen/go/` are hand-written to match what `protoc-gen-go-grpc` would generate. To switch to binary protobuf encoding (3–5× smaller messages): install protoc, run `make proto-gen`, update imports.

### 7. Bid result publishing is async (off critical path)

After the auction, `bid-results` are published to Kafka in a detached goroutine. This keeps the HTTP response time from depending on Kafka producer latency:

```go
go func() {
    s.producer.Publish(context.Background(), kafka.TopicBidResults, req.ID, result)
}()
return bidResponse, nil
```

**Risk:** If the process crashes before the goroutine fires, the bid result is lost. Acceptable for analytics/logging; not acceptable for billing. For billing events, use synchronous publish before returning the response.

---

## Observability

### Prometheus metrics

All services expose `/metrics`. Key metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `bidflock_bid_requests_total` | Counter | `ssp_id`, `status` | QPS by SSP and outcome (bid/no_bid/error) |
| `bidflock_bid_auction_duration_seconds` | Histogram | `auction_type` | End-to-end auction latency |
| `bidflock_bid_win_price_dollars` | Histogram | `campaign_id` | Clearing price distribution |
| `bidflock_grpc_request_duration_seconds` | Histogram | `service`, `method`, `status` | gRPC call latency to Budget/Scoring |
| `bidflock_budget_deductions_total` | Counter | `campaign_id`, `status` | Budget deduction outcomes |
| `bidflock_budget_remaining_dollars` | Gauge | `campaign_id` | Real-time remaining daily budget |
| `bidflock_frequency_cap_checks_total` | Counter | `result` | Frequency cap allowed/capped ratio |
| `bidflock_campaign_cache_total` | Counter | `result` | Redis campaign cache hit rate |
| `bidflock_events_processed_total` | Counter | `event_type`, `status` | Consumer throughput |

### Grafana

- URL: `http://localhost:3000` (admin / admin)
- Add Prometheus data source: `http://prometheus:9090`
- Import dashboards from `config/grafana/`

Suggested panels: QPS by SSP, P50/P95/P99 latency over time, budget burn rate, bid win rate, gRPC latency heatmap.

### Structured logs

All services log JSON to stdout with consistent field names:

```json
{
  "time": "2025-04-05T12:34:56.789Z",
  "level": "INFO",
  "service": "bidding",
  "request_id": "req-001",
  "ssp_id": "mock-adx",
  "status": "bid",
  "latency_ms": 23
}
```

Field names are defined in `pkg/observability/logging.go` to ensure consistency across services for Loki / Splunk queries.

---

## Running tests

```bash
# All unit tests
CGO_ENABLED=0 go test ./...

# With race detector (requires CGO)
go test -race ./...

# Specific package
go test -v ./internal/bidding/...
go test -v ./internal/scoring/...
```

### Test coverage

| Package | Tests | What's tested |
|---------|-------|---------------|
| `internal/bidding` | 7 tests | Second-price, first-price, floor enforcement, no-bid cases, mixed budget eligibility |
| `internal/scoring` | 5 tests | CTR output range, high vs low engagement ordering, historical CTR dominance, CVR < CTR, sigmoid correctness |
| `internal/campaign` | 6 tests | Campaign validation (name, advertiser, budget, bid, end_date), nil-publisher safety |
| `internal/simulator` | 5 tests | User profile validity, geo distribution (within ±5% of target), age distribution, reproducibility with seed, request structure |

---

## Phase 2 roadmap

| Phase | Feature | Notes |
|-------|---------|-------|
| **2a** | Ad retrieval | HNSW vector index for semantic targeting; candidate generation → auction pipeline |
| **2b** | Creative selection | Multi-armed bandit (Thompson Sampling) across creatives per campaign |
| **2c** | PID pacing controller | Replace threshold-based multiplier with proper control loop |
| **2d** | ML pipeline | Train CTR/CVR models on ClickHouse data; ONNX model serving replacing logistic regression |
| **3** | Ad serving | VAST/VPAID video, click tracking pixel, attribution API |
| **4** | User targeting | User profile service, interest graph, lookalike audiences |
| **5** | Cloud deployment | Helm charts, GKE HPA, managed Redis/Kafka/ClickHouse |

---

## License

MIT
