package store

import (
	"github.com/smancke/guble/guble"

	"github.com/stretchr/testify/assert"

	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"
)

var test1 = []byte("Test1")
var test2 = []byte("Test2")
var test3 = []byte("Test3")

func CommonTestPutGetDelete(t *testing.T, s KVStore) {
	a := assert.New(t)

	a.NoError(s.Put("s1", "a", test1))
	a.NoError(s.Put("s1", "b", test2))
	a.NoError(s.Put("s2", "a", test3))

	assertGet(a, s, "s1", "a", test1)
	assertGet(a, s, "s1", "b", test2)
	assertGet(a, s, "s2", "a", test3)
	assertGetNoExist(a, s, "no", "thing")

	// delete
	s.Delete("s1", "b")
	assertGet(a, s, "s1", "a", test1)
	assertGetNoExist(a, s, "s1", "b")
	assertGet(a, s, "s2", "a", test3)
}

func CommonBenchPutGet(b *testing.B, s KVStore) {
	a := assert.New(b)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		data := randString(20)
		s.Put("bench", data, []byte(data))
		val, exist, err := s.Get("bench", data)
		a.NoError(err)
		a.True(exist)
		a.Equal(data, string(val))
	}
	b.StopTimer()
}

func assertGet(a *assert.Assertions, s KVStore, schema string, key string, expectedValue []byte) {
	val, exist, err := s.Get(schema, key)
	a.NoError(err)
	a.True(exist)
	a.Equal(expectedValue, val)
}

func assertGetNoExist(a *assert.Assertions, s KVStore, schema string, key string) {
	val, exist, err := s.Get(schema, key)
	a.NoError(err)
	a.False(exist)
	a.Nil(val)
}

func enableDebugForMethod() func() {
	reset := guble.LogLevel
	guble.LogLevel = guble.LEVEL_DEBUG
	return func() { guble.LogLevel = reset }
}

func randString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

func tempFilename() string {
	file, err := ioutil.TempFile("/tmp", "guble_sqlite_unittest")
	if err != nil {
		panic(err)
	}
	file.Close()
	os.Remove(file.Name())
	return file.Name()
}
