package store

import (
	"testing"
)

func TestMemoryPutGetDelete(t *testing.T) {
	CommonTestPutGetDelete(t, NewMemoryKVStore())
}

func TestMemoryIterateKeys(t *testing.T) {
	mkvs := NewMemoryKVStore()
	CommonTestIterateKeys(t, mkvs, mkvs)
}

func TestMemoryIterate(t *testing.T) {
	mkvs := NewMemoryKVStore()
	CommonTestIterate(t, mkvs, mkvs)
}

func BenchmarkMemoryPutGet(b *testing.B) {
	CommonBenchmarkPutGet(b, NewMemoryKVStore())
}
