package gubled

import (
	"github.com/stretchr/testify/assert"

	"reflect"
	"testing"
)

func TestCreateStoreBackend(t *testing.T) {
	a := assert.New(t)

	memory := CreateStoreBackend(Args{KVBackend: "memory"})
	a.Equal("*store.MemoryKVStore", reflect.TypeOf(memory).String())

	sqlite := CreateStoreBackend(Args{KVBackend: "sqlite"})
	a.Equal("*store.SqliteKVStore", reflect.TypeOf(sqlite).String())
}

func TestCreateStoreBackendPanicInvalidBackend(t *testing.T) {
	a := assert.New(t)

	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		CreateStoreBackend(Args{KVBackend: "foo bar"})
	}()
	a.NotNil(p)
}
