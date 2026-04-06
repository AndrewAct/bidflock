#!/usr/bin/env bash
# loadtest.sh — Extreme stress test using k6.
# Requires k6 installed: https://k6.io/docs/get-started/installation/
# Usage: ./scripts/loadtest.sh

set -euo pipefail

BID_URL="${BID_URL:-http://localhost:8081/bid}"

if ! command -v k6 &>/dev/null; then
    echo "k6 not found. Install from https://k6.io/docs/get-started/installation/"
    exit 1
fi

cat <<'EOF' > /tmp/bidflock-k6.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const bidWinRate = new Rate('bid_wins');
const bidLatency = new Trend('bid_latency_ms');

const BID_URL = __ENV.BID_URL || 'http://localhost:8081/bid';

// Ramp up to 100K VUs over 5 minutes, hold for 10 minutes, ramp down
export const options = {
    stages: [
        { duration: '1m', target: 100 },
        { duration: '2m', target: 1000 },
        { duration: '2m', target: 5000 },
        { duration: '2m', target: 10000 },
        { duration: '2m', target: 50000 },
        { duration: '5m', target: 100000 },  // peak
        { duration: '2m', target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(99)<100'],  // 99% of requests < 100ms
        http_req_failed: ['rate<0.01'],    // error rate < 1%
        errors: ['rate<0.01'],
    },
};

function generateBidRequest() {
    const id = `req-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    const geos = ['US', 'JP', 'GB', 'DE', 'BR'];
    const oses = ['iOS', 'Android', 'Windows'];
    const ssps = ['mock-adx', 'mock-tiktok-exchange', 'mock-pangle'];

    return {
        id: id,
        imp: [{
            id: `imp-${id}`,
            banner: { w: 320, h: 50 },
            bidfloor: 0.3,
            bidfloorcur: 'USD',
        }],
        app: {
            id: 'app-001',
            name: 'com.example.app',
            bundle: 'com.example.app',
        },
        user: {
            id: `user-${Math.floor(Math.random() * 1000000)}`,
            yob: 1990 + Math.floor(Math.random() * 15),
            gender: Math.random() < 0.5 ? 'M' : 'F',
            geo: { country: geos[Math.floor(Math.random() * geos.length)] },
        },
        device: {
            os: oses[Math.floor(Math.random() * oses.length)],
            devicetype: 4,
            ip: '1.2.3.4',
        },
        at: 2,
        tmax: 150,
        ext: {
            ssp_id: ssps[Math.floor(Math.random() * ssps.length)],
        },
    };
}

export default function() {
    const payload = JSON.stringify(generateBidRequest());
    const params = {
        headers: { 'Content-Type': 'application/json' },
        timeout: '200ms',
    };

    const start = Date.now();
    const res = http.post(BID_URL, payload, params);
    bidLatency.add(Date.now() - start);

    const ok = check(res, {
        'status is 200': (r) => r.status === 200,
        'has id': (r) => {
            try { return JSON.parse(r.body).id !== undefined; }
            catch { return false; }
        },
    });

    errorRate.add(!ok);

    if (res.status === 200) {
        try {
            const body = JSON.parse(res.body);
            bidWinRate.add(body.seatbid && body.seatbid.length > 0);
        } catch {}
    }
}
EOF

echo "Running k6 stress test against $BID_URL"
echo "Results will show in terminal. Grafana dashboard: http://localhost:3000"
echo ""

k6 run /tmp/bidflock-k6.js -e BID_URL="$BID_URL"
