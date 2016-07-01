package store

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func BenchmarkSqlitePutGet(b *testing.B) {
	f := tempFilename()
	defer os.Remove(f)

	db := NewSqliteKVStore(f, false)
	db.Open()
	CommonBenchmarkPutGet(b, db)
}

func TestSqlitePutGetDelete(t *testing.T) {
	f := tempFilename()
	defer os.Remove(f)

	db := NewSqliteKVStore(f, false)
	db.Open()
	CommonTestPutGetDelete(t, db)
}

func TestSqliteIterate(t *testing.T) {
	f := tempFilename()
	defer os.Remove(f)

	db := NewSqliteKVStore(f, false)
	db.Open()

	CommonTestIterate(t, db, db)
}

func TestSqliteIterateKeys(t *testing.T) {
	f := tempFilename()
	defer os.Remove(f)

	db := NewSqliteKVStore(f, false)
	db.Open()

	CommonTestIterateKeys(t, db, db)
}

func TestCheck_SqlKVStore(t *testing.T) {
	a := assert.New(t)
	f := tempFilename()
	defer os.Remove(f)

	kvs := NewSqliteKVStore(f, false)
	kvs.Open()

	err := kvs.Check()
	a.Nil(err, "Db ping should work")

	kvs.Stop()

	err = kvs.Check()
	a.NotNil(err, "Check should fail because db was already closed")
}
