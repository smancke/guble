package store

import (
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func BenchmarkPostgresPutGet(b *testing.B) {
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonBenchmarkPutGet(b, db)
}

func TestPostgresPutGetDelete(t *testing.T) {
	testutil.SkipIfShort(t)
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestPutGetDelete(t, db)
}

func TestPostgresIterate(t *testing.T) {
	testutil.SkipIfShort(t)
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestIterate(t, db)
}

func TestPostgresIterateKeys(t *testing.T) {
	testutil.SkipIfShort(t)
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestIterateKeys(t, db)
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

func aPostgresConfig() PostgresConfig {
	return PostgresConfig{
		map[string]string{
			"host":     "localhost",
			"user":     "guble",
			"password": "guble",
			"dbname":   "guble",
			"sslmode":  "disable",
		},
		1,
		1,
	}
}
