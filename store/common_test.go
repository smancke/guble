package store

import (
	"github.com/stretchr/testify/assert"

	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"
	"time"
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

	s.Delete("s1", "b")
	assertGet(a, s, "s1", "a", test1)
	assertGetNoExist(a, s, "s1", "b")
	assertGet(a, s, "s2", "a", test3)
}

func CommonTestIterate(t *testing.T, s KVStore) {
	a := assert.New(t)

	a.NoError(s.Put("s1", "bli", test1))
	a.NoError(s.Put("s1", "bla", test2))
	a.NoError(s.Put("s1", "buu", test3))
	a.NoError(s.Put("s2", "bli", test2))

	assertChannelContainsEntries(a, s.Iterate("s1", "bl"),
		[2]string{"bli", string(test1)},
		[2]string{"bla", string(test2)})

	assertChannelContainsEntries(a, s.Iterate("s1", ""),
		[2]string{"bli", string(test1)},
		[2]string{"bla", string(test2)},
		[2]string{"buu", string(test3)})

	assertChannelContainsEntries(a, s.Iterate("s1", "bla"),
		[2]string{"bla", string(test2)})

	assertChannelContainsEntries(a, s.Iterate("s1", "nothing"))

	assertChannelContainsEntries(a, s.Iterate("s2", ""),
		[2]string{"bli", string(test2)})
}

func assertChannelContainsEntries(a *assert.Assertions, entryC chan [2]string, expectedEntries ...[2]string) {
	allEntries := make([][2]string, 0)

WAITLOOP:
	for {
		select {
		case entry, ok := <-entryC:
			if !ok {
				break WAITLOOP
			}
			allEntries = append(allEntries, entry)
		case <-time.After(time.Second):
			a.Fail("timeout")
		}
	}

	a.Equal(len(expectedEntries), len(allEntries))
	for _, expected := range expectedEntries {
		a.Contains(allEntries, expected)
	}
}

func CommonTestIterateKeys(t *testing.T, s KVStore) {
	a := assert.New(t)

	a.NoError(s.Put("s1", "bli", test1))
	a.NoError(s.Put("s1", "bla", test2))
	a.NoError(s.Put("s1", "buu", test3))
	a.NoError(s.Put("s2", "bli", test2))

	assertChannelContains(a, s.IterateKeys("s1", "bl"),
		"bli", "bla")

	assertChannelContains(a, s.IterateKeys("s1", ""),
		"bli", "bla", "buu")

	assertChannelContains(a, s.IterateKeys("s1", "bla"),
		"bla")

	assertChannelContains(a, s.IterateKeys("s1", "nothing"))

	assertChannelContains(a, s.IterateKeys("s2", ""),
		"bli")
}

func assertChannelContains(a *assert.Assertions, entryC chan string, expectedEntries ...string) {
	allEntries := make([]string, 0)

WAITLOOP:
	for {
		select {
		case entry, ok := <-entryC:
			if !ok {
				break WAITLOOP
			}
			allEntries = append(allEntries, entry)
		case <-time.After(time.Second):
			a.Fail("timeout")
		}
	}

	a.Equal(len(expectedEntries), len(allEntries))
	for _, expected := range expectedEntries {
		a.Contains(allEntries, expected)
	}
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
	file, err := ioutil.TempFile("/tmp", "guble_store_unittest")
	if err != nil {
		panic(err)
	}
	file.Close()
	os.Remove(file.Name())
	return file.Name()
}
