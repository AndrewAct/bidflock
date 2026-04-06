package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// BatchWriter buffers events and flushes them to ClickHouse in batches.
// Flushes on size threshold or time interval, whichever comes first.
type BatchWriter struct {
	conn      driver.Conn
	batchSize int
	interval  time.Duration
	logger    *slog.Logger

	mu      sync.Mutex
	batches map[string][]interface{} // table -> rows
	timer   *time.Timer
}

func NewBatchWriter(dsn string, batchSize int, flushInterval time.Duration, logger *slog.Logger) (*BatchWriter, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},
		Auth: clickhouse.Auth{
			Database: "bidflock",
			Username: "default",
			Password: "",
		},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse connect: %w", err)
	}

	bw := &BatchWriter{
		conn:      conn,
		batchSize: batchSize,
		interval:  flushInterval,
		logger:    logger,
		batches:   make(map[string][]interface{}),
	}
	bw.timer = time.AfterFunc(flushInterval, bw.flushAll)
	return bw, nil
}

func (bw *BatchWriter) WriteImpression(ctx context.Context, row ImpressionRow) {
	bw.append("impressions", row)
}

func (bw *BatchWriter) WriteClick(ctx context.Context, row ClickRow) {
	bw.append("clicks", row)
}

func (bw *BatchWriter) WriteConversion(ctx context.Context, row ConversionRow) {
	bw.append("conversions", row)
}

func (bw *BatchWriter) WriteBidResult(ctx context.Context, row BidResultRow) {
	bw.append("bid_results", row)
}

func (bw *BatchWriter) append(table string, row interface{}) {
	bw.mu.Lock()
	bw.batches[table] = append(bw.batches[table], row)
	size := len(bw.batches[table])
	bw.mu.Unlock()

	if size >= bw.batchSize {
		go bw.flush(table)
	}
}

func (bw *BatchWriter) flushAll() {
	bw.mu.Lock()
	tables := make([]string, 0, len(bw.batches))
	for t := range bw.batches {
		tables = append(tables, t)
	}
	bw.mu.Unlock()

	for _, t := range tables {
		bw.flush(t)
	}
	bw.timer.Reset(bw.interval)
}

func (bw *BatchWriter) flush(table string) {
	bw.mu.Lock()
	rows := bw.batches[table]
	bw.batches[table] = nil
	bw.mu.Unlock()

	if len(rows) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := bw.insertBatch(ctx, table, rows); err != nil {
		bw.logger.Error("clickhouse flush failed", "table", table, "rows", len(rows), "err", err)
	} else {
		bw.logger.Info("flushed to clickhouse", "table", table, "rows", len(rows))
	}
}

func (bw *BatchWriter) insertBatch(ctx context.Context, table string, rows []interface{}) error {
	batch, err := bw.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s", table))
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.AppendStruct(row); err != nil {
			return err
		}
	}
	return batch.Send()
}

// Close flushes all pending batches and closes the connection.
func (bw *BatchWriter) Close() {
	bw.timer.Stop()
	bw.flushAll()
	bw.conn.Close()
}

// ClickHouse row types — match the DDL in deployments/

type ImpressionRow struct {
	EventID    string    `ch:"event_id"`
	BidID      string    `ch:"bid_id"`
	RequestID  string    `ch:"request_id"`
	CampaignID string    `ch:"campaign_id"`
	AdID       string    `ch:"ad_id"`
	UserID     string    `ch:"user_id"`
	SSPID      string    `ch:"ssp_id"`
	Price      float64   `ch:"price"`
	Timestamp  time.Time `ch:"timestamp"`
}

type ClickRow struct {
	EventID      string    `ch:"event_id"`
	ImpressionID string    `ch:"impression_id"`
	CampaignID   string    `ch:"campaign_id"`
	AdID         string    `ch:"ad_id"`
	UserID       string    `ch:"user_id"`
	Timestamp    time.Time `ch:"timestamp"`
}

type ConversionRow struct {
	EventID    string    `ch:"event_id"`
	ClickID    string    `ch:"click_id"`
	CampaignID string    `ch:"campaign_id"`
	AdID       string    `ch:"ad_id"`
	UserID     string    `ch:"user_id"`
	Value      float64   `ch:"value"`
	Timestamp  time.Time `ch:"timestamp"`
}

type BidResultRow struct {
	RequestID    string    `ch:"request_id"`
	AuctionType  string    `ch:"auction_type"`
	WinnerCamID  string    `ch:"winner_campaign_id"`
	WinnerAdID   string    `ch:"winner_ad_id"`
	ClearPrice   float64   `ch:"clearing_price"`
	NoBid        bool      `ch:"no_bid"`
	NoBidReason  int32     `ch:"no_bid_reason"`
	DurationUS   int64     `ch:"duration_us"`
	SSPID        string    `ch:"ssp_id"`
	Timestamp    time.Time `ch:"timestamp"`
}
