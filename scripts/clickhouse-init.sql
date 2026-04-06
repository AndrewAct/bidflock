CREATE DATABASE IF NOT EXISTS bidflock;

CREATE TABLE IF NOT EXISTS bidflock.bid_results (
    request_id       String,
    auction_type     String,
    winner_campaign_id String,
    winner_ad_id     String,
    clearing_price   Float64,
    no_bid           Bool,
    no_bid_reason    Int32,
    duration_us      Int64,
    ssp_id           String,
    timestamp        DateTime64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, ssp_id, winner_campaign_id)
TTL timestamp + INTERVAL 90 DAY;

CREATE TABLE IF NOT EXISTS bidflock.impressions (
    event_id    String,
    bid_id      String,
    request_id  String,
    campaign_id String,
    ad_id       String,
    user_id     String,
    ssp_id      String,
    price       Float64,
    timestamp   DateTime64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, campaign_id, ad_id)
TTL timestamp + INTERVAL 90 DAY;

CREATE TABLE IF NOT EXISTS bidflock.clicks (
    event_id      String,
    impression_id String,
    campaign_id   String,
    ad_id         String,
    user_id       String,
    timestamp     DateTime64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, campaign_id)
TTL timestamp + INTERVAL 90 DAY;

CREATE TABLE IF NOT EXISTS bidflock.conversions (
    event_id    String,
    click_id    String,
    campaign_id String,
    ad_id       String,
    user_id     String,
    value       Float64,
    timestamp   DateTime64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, campaign_id)
TTL timestamp + INTERVAL 90 DAY;

-- Analytics views
CREATE VIEW IF NOT EXISTS bidflock.campaign_performance AS
SELECT
    i.campaign_id,
    countIf(b.no_bid = false)   AS total_bids,
    count(i.event_id)           AS impressions,
    count(c.event_id)           AS clicks,
    count(cv.event_id)          AS conversions,
    round(count(c.event_id) / nullIf(count(i.event_id), 0) * 100, 3) AS ctr_pct,
    round(count(cv.event_id) / nullIf(count(c.event_id), 0) * 100, 3) AS cvr_pct,
    round(sum(i.price) / 1000, 2) AS spend_usd,
    round(sum(cv.value), 2)     AS revenue_usd
FROM bidflock.impressions i
LEFT JOIN bidflock.clicks c ON i.event_id = c.impression_id
LEFT JOIN bidflock.conversions cv ON c.event_id = cv.click_id
LEFT JOIN bidflock.bid_results b ON i.request_id = b.request_id
GROUP BY i.campaign_id;
