package database

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedisDB_Connect_InvalidDSN(t *testing.T) {
	t.Run("returns error for unparseable DSN", func(t *testing.T) {
		db := NewRedisDB()
		err := db.Connect(context.Background(), "not-a-valid-dsn")
		require.Error(t, err)
	})
}

func TestRedisDB_Ping_BeforeConnect(t *testing.T) {
	t.Run("returns error when not connected", func(t *testing.T) {
		db := NewRedisDB()
		err := db.Ping(context.Background())
		require.ErrorIs(t, err, errNotConnected)
	})
}

func TestRedisDB_Stats_BeforeConnect(t *testing.T) {
	t.Run("returns zero stats when not connected", func(t *testing.T) {
		db := NewRedisDB()
		require.Equal(t, RedisStats{}, db.Stats())
	})
}

func TestRedisDB_Client_BeforeConnect(t *testing.T) {
	t.Run("returns nil when not connected", func(t *testing.T) {
		db := NewRedisDB()
		require.Nil(t, db.Client())
	})
}

func TestRedisDB_Close_BeforeConnect(t *testing.T) {
	t.Run("does not error when not connected", func(t *testing.T) {
		db := NewRedisDB()
		require.NoError(t, db.Close())
	})
}

func TestRedisDB_Connect_Race(t *testing.T) {
	t.Run("concurrent Connect calls on same instance do not race", func(t *testing.T) {
		db := NewRedisDB()
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				db.Connect(context.Background(), "redis://localhost:6379/0") //nolint:errcheck
			})
		}
		wg.Wait()
		db.Close() //nolint:errcheck
	})
}

func TestRedisDB_ReadMethods_Race(t *testing.T) {
	t.Run("concurrent read methods before connect do not race", func(t *testing.T) {
		db := NewRedisDB()
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				db.Ping(context.Background()) //nolint:errcheck
				db.Stats()
				db.Client()
			})
		}
		wg.Wait()
	})
}

func TestRedisDB_ConnectAndRead_Race(t *testing.T) {
	t.Run("concurrent Connect and read methods do not race", func(t *testing.T) {
		db := NewRedisDB()
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				db.Connect(context.Background(), "redis://localhost:6379/0") //nolint:errcheck
			})
		}
		for range 10 {
			wg.Go(func() {
				db.Ping(context.Background()) //nolint:errcheck
				db.Stats()
				db.Client()
			})
		}
		wg.Wait()
		db.Close() //nolint:errcheck
	})
}
