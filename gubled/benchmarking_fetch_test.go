package gubled

import (
	"github.com/smancke/guble/client"

	"github.com/stretchr/testify/assert"

	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
)

func Benchmark_E2E_Fetch_HelloWorld_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_benchmark_test")
	defer os.RemoveAll(dir)

	service := StartupService(Args{Listen: "localhost:0", KVBackend: "memory", MSBackend: "file", StoragePath: dir})
	defer service.Stop()

	time.Sleep(time.Millisecond * 10)

	// fill the topic
	location := "ws://" + service.WebServer().GetAddr() + "/stream/user/xy"
	c, err := client.Open(location, "http://localhost/", 1000, true)
	a.NoError(err)

	for i := 1; i <= b.N; i++ {
		a.NoError(c.Send("/hello", fmt.Sprintf("Hello %v", i), ""))
		select {
		case <-c.StatusMessages():
			// wait for, but ignore
		case <-time.After(time.Millisecond * 50):
			a.Fail("timeout on send notification")
			return
		}
	}

	start := time.Now()
	b.ResetTimer()
	c.WriteRawMessage([]byte("+ /hello 0 1000000"))
	for i := 1; i <= b.N; i++ {
		select {
		case msg := <-c.Messages():
			a.Equal(fmt.Sprintf("Hello %v", i), msg.BodyAsString())
		case e := <-c.Errors():
			a.Fail(string(e.Bytes()))
			return
		case <-time.After(time.Second):
			a.Fail("timeout on message: " + strconv.Itoa(i))
			return
		}
	}
	b.StopTimer()

	end := time.Now()
	throughput := float64(b.N) / end.Sub(start).Seconds()
	fmt.Printf("\n\tThroughput: %v/sec (%v message in %v)\n", int(throughput), b.N, end.Sub(start))
}
