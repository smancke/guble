package store

import (
	"os"
	"testing"
)

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

	CommonTestIterate(t, db)
}

func TestSqliteIterateKeys(t *testing.T) {
	f := tempFilename()
	defer os.Remove(f)

	db := NewSqliteKVStore(f, false)
	db.Open()

	CommonTestIterateKeys(t, db)
}

func BenchmarkSqlitePutGet(b *testing.B) {
	f := tempFilename()
	defer os.Remove(f)

	db := NewSqliteKVStore(f, false)
	db.Open()
	CommonBenchPutGet(b, db)
}
