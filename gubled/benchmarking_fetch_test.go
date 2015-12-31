package gubled

import (
	"github.com/smancke/guble/client"

	"github.com/stretchr/testify/assert"

	"fmt"
	"io/ioutil"
	"strconv"
	"testing"
	"time"
)

func Benchmark_E2E_Fetch_HelloWorld_Messages(b *testing.B) {
	a := assert.New(b)

	dir, _ := ioutil.TempDir("", "guble_benchmark_test")
	//defer os.RemoveAll(dir)

	service := StartupService(Args{Listen: "localhost:0", KVBackend: "memory", MSBackend: "file", MSPath: dir})
	defer service.Stop()

	time.Sleep(time.Millisecond * 10)

	// fill the topic
	location := "ws://" + service.GetWebServer().GetAddr() + "/stream/user/xy"
	c, err := client.Open(location, "http://localhost/", 1000, false)
	a.NoError(err)

	for i := 1; i <= b.N; i++ {
		a.NoError(c.Send("/hello", fmt.Sprintf("Hello %v", i), ""))
	}

	start := time.Now()
	b.ResetTimer()
	c.WriteRawMessage([]byte("replay /hello"))
	for i := 1; i <= b.N; i++ {
		select {
		case msg := <-c.Messages():
			a.Equal(fmt.Sprintf("Hello %v", i), msg.BodyAsString())
		case error := <-c.Errors():
			a.Fail(string(error.Bytes()))
			return
		case <-c.StatusMessages():
			// ignore
		case <-time.After(time.Millisecond * 50):
			a.Fail("timeout on message: " + strconv.Itoa(i))
			return
		}
	}
	b.StopTimer()

	end := time.Now()
	throughput := float64(b.N) / end.Sub(start).Seconds()
	fmt.Printf("\n\tThroughput: %v/sec (%v message in %v)\n", int(throughput), b.N, end.Sub(start))
}
