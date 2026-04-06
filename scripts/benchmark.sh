#!/usr/bin/env bash
# benchmark.sh — Runs the simulator at increasing QPS levels and records latency curves.
# Output: benchmark-results/ directory with JSON+CSV files for each QPS level.
# Usage: ./scripts/benchmark.sh [--bid-url URL] [--output DIR]

set -euo pipefail

BID_URL="${BID_URL:-http://localhost:8081/bid}"
OUTPUT_DIR="${OUTPUT_DIR:-./benchmark-results}"
DURATION="${DURATION:-30s}"

mkdir -p "$OUTPUT_DIR"

QPS_LEVELS=(100 500 1000 2000 5000 10000 20000 50000)

echo "bidflock benchmark run"
echo "bid URL: $BID_URL"
echo "output: $OUTPUT_DIR"
echo "duration per level: $DURATION"
echo ""
echo "QPS levels: ${QPS_LEVELS[*]}"
echo ""

for qps in "${QPS_LEVELS[@]}"; do
    echo "─────────────────────────────────"
    echo "Running at $qps QPS for $DURATION..."

    go run ./cmd/simulator/main.go \
        --qps="$qps" \
        --duration="$DURATION" \
        --pattern=steady \
        --bid-url="$BID_URL" \
        --output="$OUTPUT_DIR" \
        --seed=42

    echo "Done at $qps QPS."
    sleep 5  # cooldown between levels
done

echo ""
echo "All benchmark runs complete."
echo "Results in: $OUTPUT_DIR"
echo "Combine CSVs with: cat $OUTPUT_DIR/*.csv | sort -t, -k1 -n"
