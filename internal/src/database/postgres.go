package database

import (
	"context"
	"errors"
	"sync"

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
