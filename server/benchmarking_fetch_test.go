package server

import (
	"github.com/cosminrentea/gobbler/client"
	"github.com/cosminrentea/gobbler/testutil"

	"github.com/stretchr/testify/assert"

	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
	"github.com/cosminrentea/gobbler/server/configstring"
)

func Benchmark_E2E_Fetch_HelloWorld_Messages(b *testing.B) {
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_benchmarking_fetch_test")
	defer os.RemoveAll(dir)

	*Config.HttpListen = "localhost:0"
	*Config.KVS = "memory"
	*Config.MS = "file"
	*Config.StoragePath = dir
	*Config.WS.Enabled = true
	*Config.WS.Prefix = "/stream/"
	*Config.KafkaProducer.Brokers = configstring.List{}

	service := StartService()
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
		case <-time.After(time.Millisecond * 100):
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
			a.Equal(fmt.Sprintf("Hello %v", i), string(msg.Body))
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
