package database

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotConnected = errors.New("database: not connected")

type PostgresStats struct {
	TotalConns        int32
	AcquiredConns     int32
	IdleConns         int32
	MaxConns          int32
	ConstructingConns int32
}

type PostgresDB interface {
	Connect(ctx context.Context, dsn string) error
	Ping(ctx context.Context) error
	Close()
	Pool() *pgxpool.Pool
	Stats() PostgresStats
}

type postgresDB struct {
	mu   sync.RWMutex
	pool *pgxpool.Pool
}

func NewPostgresDB() PostgresDB {
	return &postgresDB{}
}

func (p *postgresDB) Connect(ctx context.Context, dsn string) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.pool = pool
	p.mu.Unlock()
	return nil
}

func (p *postgresDB) Ping(ctx context.Context) error {
	p.mu.RLock()
	pool := p.pool
	p.mu.RUnlock()
	if pool == nil {
		return errNotConnected
	}
	return pool.Ping(ctx)
}

func (p *postgresDB) Close() {
	p.mu.Lock()
	pool := p.pool
	p.pool = nil
	p.mu.Unlock()
	if pool != nil {
		pool.Close()
	}
}

func (p *postgresDB) Pool() *pgxpool.Pool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pool
}

func (p *postgresDB) Stats() PostgresStats {
	p.mu.RLock()
	pool := p.pool
	p.mu.RUnlock()
	if pool == nil {
		return PostgresStats{}
	}
	s := pool.Stat()
	return PostgresStats{
		TotalConns:        s.TotalConns(),
		AcquiredConns:     s.AcquiredConns(),
		IdleConns:         s.IdleConns(),
		MaxConns:          s.MaxConns(),
		ConstructingConns: s.ConstructingConns(),
	}
}

// PingEventually pings the database on a fixed interval until ctx is cancelled.
// Logs pool stats after each successful ping.
func PingPostgresEventually(ctx context.Context, db PostgresDB, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := db.Ping(ctx); err != nil {
					slog.Error("postgres ping failed", "error", err)
					continue
				}
				s := db.Stats()
				slog.Info("postgres pool stats",
					"total_conns", s.TotalConns,
					"acquired_conns", s.AcquiredConns,
					"idle_conns", s.IdleConns,
					"max_conns", s.MaxConns,
				)
			}
		}
	}()
}
