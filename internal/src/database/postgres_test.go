package database

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresDB_Connect_InvalidDSN(t *testing.T) {
	t.Run("returns error for unparseable DSN", func(t *testing.T) {
		db := NewPostgresDB()
		err := db.Connect(context.Background(), "not-a-valid-dsn://")
		require.Error(t, err)
	})
}

func TestPostgresDB_Ping_BeforeConnect(t *testing.T) {
	t.Run("returns error when not connected", func(t *testing.T) {
		db := NewPostgresDB()
		err := db.Ping(context.Background())
		require.ErrorIs(t, err, errNotConnected)
	})
}

func TestPostgresDB_Stats_BeforeConnect(t *testing.T) {
	t.Run("returns zero stats when not connected", func(t *testing.T) {
		db := NewPostgresDB()
		require.Equal(t, PostgresStats{}, db.Stats())
	})
}

func TestPostgresDB_Pool_BeforeConnect(t *testing.T) {
	t.Run("returns nil when not connected", func(t *testing.T) {
		db := NewPostgresDB()
		require.Nil(t, db.Pool())
	})
}

func TestPostgresDB_Close_BeforeConnect(t *testing.T) {
	t.Run("does not panic when not connected", func(t *testing.T) {
		db := NewPostgresDB()
		require.NotPanics(t, func() { db.Close() })
	})
}

func TestPostgresDB_Connect_Race(t *testing.T) {
	t.Run("concurrent Connect calls on same instance do not race", func(t *testing.T) {
		db := NewPostgresDB()
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				db.Connect(context.Background(), "not-a-dsn") //nolint:errcheck
			})
		}
		wg.Wait()
	})
}

func TestPostgresDB_ReadMethods_Race(t *testing.T) {
	t.Run("concurrent read methods before connect do not race", func(t *testing.T) {
		db := NewPostgresDB()
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				db.Ping(context.Background()) //nolint:errcheck
				db.Stats()
				db.Pool()
			})
		}
		wg.Wait()
	})
}

func TestPostgresDB_ConnectAndRead_Race(t *testing.T) {
	t.Run("concurrent Connect and read methods do not race", func(t *testing.T) {
		db := NewPostgresDB()
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				db.Connect(context.Background(), "not-a-dsn") //nolint:errcheck
			})
		}
		for range 10 {
			wg.Go(func() {
				db.Ping(context.Background()) //nolint:errcheck
				db.Stats()
				db.Pool()
			})
		}
		wg.Wait()
	})
}
