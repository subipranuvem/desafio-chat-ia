package database

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStats struct {
	TotalConns uint32
	IdleConns  uint32
	StaleConns uint32
	Hits       uint32
	Misses     uint32
	Timeouts   uint32
}

type RedisDB interface {
	Connect(ctx context.Context, dsn string) error
	Ping(ctx context.Context) error
	Close() error
	Client() *redis.Client
	Stats() RedisStats
}

type redisDB struct {
	mu     sync.RWMutex
	client *redis.Client
}

func NewRedisDB() RedisDB {
	return &redisDB{}
}

func (r *redisDB) Connect(_ context.Context, dsn string) error {
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return err
	}
	client := redis.NewClient(opts)
	r.mu.Lock()
	r.client = client
	r.mu.Unlock()
	return nil
}

func (r *redisDB) Ping(ctx context.Context) error {
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	if client == nil {
		return errNotConnected
	}
	return client.Ping(ctx).Err()
}

func (r *redisDB) Close() error {
	r.mu.Lock()
	client := r.client
	r.client = nil
	r.mu.Unlock()
	if client == nil {
		return nil
	}
	return client.Close()
}

func (r *redisDB) Client() *redis.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

func (r *redisDB) Stats() RedisStats {
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	if client == nil {
		return RedisStats{}
	}
	s := client.PoolStats()
	return RedisStats{
		TotalConns: s.TotalConns,
		IdleConns:  s.IdleConns,
		StaleConns: s.StaleConns,
		Hits:       s.Hits,
		Misses:     s.Misses,
		Timeouts:   s.Timeouts,
	}
}

// PingRedisEventually pings Redis on a fixed interval until ctx is cancelled.
// Logs pool stats after each successful ping.
func PingRedisEventually(ctx context.Context, db RedisDB, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := db.Ping(ctx); err != nil {
					slog.Error("redis ping failed", "error", err)
					continue
				}
				s := db.Stats()
				slog.Info("redis pool stats",
					"total_conns", s.TotalConns,
					"idle_conns", s.IdleConns,
					"hits", s.Hits,
					"misses", s.Misses,
					"timeouts", s.Timeouts,
				)
			}
		}
	}()
}
