package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DB numbers for logical isolation
const (
	DBCampaignCache = 0
	DBBudget        = 1
	DBFrequencyCap  = 2
	DBFeatureStore  = 3
	DBAuction       = 4
)

type Client struct {
	rdb *redis.Client
}

func NewClient(addr string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		DB:           db,
		PoolSize:     50,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	return &Client{rdb: rdb}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

func (c *Client) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *Client) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.rdb.IncrBy(ctx, key, value).Result()
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// RunScript executes a Lua script atomically.
func (c *Client) RunScript(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return c.rdb.Eval(ctx, script, keys, args...).Result()
}

// ZAdd adds members to a sorted set (used for frequency capping).
func (c *Client) ZAdd(ctx context.Context, key string, score float64, member string) error {
	return c.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZCount counts members in a sorted set score range.
func (c *Client) ZCount(ctx context.Context, key string, min, max string) (int64, error) {
	return c.rdb.ZCount(ctx, key, min, max).Result()
}

// ZRemRangeByScore removes members by score range (used for sliding window cleanup).
func (c *Client) ZRemRangeByScore(ctx context.Context, key string, min, max string) error {
	return c.rdb.ZRemRangeByScore(ctx, key, min, max).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// Raw returns the underlying redis.Client for operations not wrapped here.
func (c *Client) Raw() *redis.Client {
	return c.rdb
}
