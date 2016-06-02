package store

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMemoryPutGetDelete(t *testing.T) {
	CommonTestPutGetDelete(t, NewMemoryKVStore())
}

func TestMemoryIterateKeys(t *testing.T) {
	CommonTestIterateKeys(t, NewMemoryKVStore())
}

func TestMemoryIterate(t *testing.T) {
	CommonTestIterate(t, NewMemoryKVStore())
}

func BenchmarkMemoryPutGet(b *testing.B) {
	CommonBenchPutGet(b, NewMemoryKVStore())
}

func Test_Check_MemoryKvStore(t *testing.T) {
	a := assert.New(t)
	store := NewMemoryKVStore()

	err := store.Check()
	a.Nil(err)
}
