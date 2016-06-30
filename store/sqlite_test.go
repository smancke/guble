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

	CommonTestIterate(t, db)
}

func TestSqliteIterateKeys(t *testing.T) {
	f := tempFilename()
	defer os.Remove(f)

	db := NewSqliteKVStore(f, false)
	db.Open()

	CommonTestIterateKeys(t, db)
}

func TestCheck_SqlKVStore(t *testing.T) {
	a := assert.New(t)
	f := tempFilename()
	defer os.Remove(f)

	store := NewSqliteKVStore(f, false)
	//start the DB
	store.Open()

	//check should work
	err := store.Check()
	a.Nil(err, "Db ping should work")

	//close DB
	store.Stop()

	//check should throw an error
	err = store.Check()
	a.NotNil(err, "Db ping should not work. Db is closed")
}
