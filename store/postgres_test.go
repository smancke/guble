package store

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPostgresPutGetDelete(t *testing.T) {
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestPutGetDelete(t, db)
}

func TestPostgresIterate(t *testing.T) {
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestIterate(t, db)
}

func TestPostgresIterateKeys(t *testing.T) {
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonTestIterateKeys(t, db)
}

func BenchmarkPostgresPutGet(b *testing.B) {
	db := NewPostgresKVStore(aPostgresConfig())
	db.Open()
	CommonBenchPutGet(b, db)
}

func TestPostgresKVStore_Check(t *testing.T) {
	a := assert.New(t)

	kvs := NewPostgresKVStore(aPostgresConfig())
	kvs.Open()

	//check should work
	err := kvs.Check()
	a.NoError(err, "Db ping should work")

	kvs.Stop()

	//check should throw an error
	err = kvs.Check()
	a.NotNil(err, "Db ping should not work. Db is closed")
}

func aPostgresConfig() PostgresConfig {
	return PostgresConfig{
		"host":     "localhost",
		"user":     "guble",
		"password": "guble",
		"dbname":   "guble",
		"sslmode":  "disable",
	}
}
