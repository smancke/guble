package kvstore

import (
	"testing"

	"github.com/cosminrentea/gobbler/testutil"

	"github.com/stretchr/testify/assert"
)

func BenchmarkPostgresKVStore_PutGet(b *testing.B) {
	kvs := NewPostgresKVStore(aPostgresConfig())
	kvs.Open()
	CommonBenchmarkPutGet(b, kvs)
}

func TestPostgresKVStore_PutGetDelete(t *testing.T) {
	testutil.SkipIfShort(t)
	kvs := NewPostgresKVStore(aPostgresConfig())
	kvs.Open()
	CommonTestPutGetDelete(t, kvs, kvs)
}

func TestPostgresKVStore_Iterate(t *testing.T) {
	testutil.SkipIfShort(t)
	kvs := NewPostgresKVStore(aPostgresConfig())
	kvs.Open()
	CommonTestIterate(t, kvs, kvs)
}

func TestPostgresKVStore_IterateKeys(t *testing.T) {
	testutil.SkipIfShort(t)
	kvs := NewPostgresKVStore(aPostgresConfig())
	kvs.Open()
	CommonTestIterateKeys(t, kvs, kvs)
}

func TestPostgresKVStore_Check(t *testing.T) {
	testutil.SkipIfShort(t)
	a := assert.New(t)

	kvs := NewPostgresKVStore(aPostgresConfig())
	kvs.Open()

	err := kvs.Check()
	a.NoError(err, "Db ping should work")

	kvs.Stop()

	err = kvs.Check()
	a.NotNil(err, "Check should fail because db was already closed")
}

func TestPostgresKVStore_Open(t *testing.T) {
	testutil.SkipIfShort(t)
	kvs := NewPostgresKVStore(invalidPostgresConfig())
	err := kvs.Open()
	assert.NotNil(t, err)
}

// This config assumes a postgresql running locally
func aPostgresConfig() PostgresConfig {
	return PostgresConfig{
		ConnParams: map[string]string{
			"host":     "localhost",
			"user":     "postgres",
			"password": "",
			"dbname":   "guble",
			"sslmode":  "disable",
		},
		MaxIdleConns: 1,
		MaxOpenConns: 1,
	}
}

func invalidPostgresConfig() PostgresConfig {
	return PostgresConfig{
		ConnParams: map[string]string{
			"host":     "localhost",
			"user":     "",
			"password": "",
			"dbname":   "",
			"sslmode":  "disable",
		},
		MaxIdleConns: 1,
		MaxOpenConns: 1,
	}
}
