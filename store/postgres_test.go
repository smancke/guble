package store

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func BenchmarkPostgresPutGet(b *testing.B) {
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonBenchmarkPutGet(b, db)
}

func TestPostgresPutGetDelete(t *testing.T) {
	skipIfShort(t)
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestPutGetDelete(t, db)
}

func TestPostgresIterate(t *testing.T) {
	skipIfShort(t)
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestIterate(t, db)
}

func TestPostgresIterateKeys(t *testing.T) {
	skipIfShort(t)
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestIterateKeys(t, db)
}

func TestPostgresKVStore_Check(t *testing.T) {
	skipIfShort(t)
	a := assert.New(t)

	kvs := NewPostgresKVStore(aPostgresConfig())
	kvs.Open()

	//check should work
	err := kvs.Check()
	a.NoError(err, "Db ping should work")

	kvs.Stop()

	//check should throw an error, after the KVStore is stopped
	err = kvs.Check()
	a.NotNil(err, "Db ping should not work. Db is closed")
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

func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
}
