package store

import (
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
	CommonBenchmarkPutGet(b, NewMemoryKVStore())
}
