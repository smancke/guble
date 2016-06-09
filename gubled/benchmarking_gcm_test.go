package gubled

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"

	"github.com/stretchr/testify/assert"

	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func BenchmarkGCMConnector_BroadcastMessagesSingleWorker(b *testing.B) {
	throughput := throughputSend(b, 1, sendBroadcastSample)
	fmt.Printf("Broadcast throughput for single worker: %.2f msg/sec\n", throughput)
}

func BenchmarkGCMConnector_BroadcastMessagesMultipleWorkers(b *testing.B) {
	gomaxprocs := runtime.GOMAXPROCS(0)
	throughput := throughputSend(b, gomaxprocs, sendBroadcastSample)
	fmt.Printf("Broadcast throughput for GOMAXPROCS (%v): %.2f msg/sec\n", gomaxprocs, throughput)
}

func BenchmarkGCMConnector_SendMessagesSingleWorker(b *testing.B) {
	throughput := throughputSend(b, 1, sendMessageSample)
	fmt.Printf("Send throughput for single worker: %.2f msg/sec\n", throughput)
}

func BenchmarkGCMConnector_SendMessagesMultipleWorkers(b *testing.B) {
	gomaxprocs := runtime.GOMAXPROCS(0)
	throughput := throughputSend(b, gomaxprocs, sendMessageSample)
	fmt.Printf("Send throughput for GOMAXPROCS (%v): %.2f msg/sec\n", gomaxprocs, throughput)
}

func throughputSend(b *testing.B, nWorkers int, sampleSend func(c client.Client) error) float64 {
	//testutil.EnableDebugForMethod()
	fmt.Printf("b.N=%v\n", b.N)
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(b)

	dir, errTempDir := ioutil.TempDir("", "guble_benchmarking_gcm_test")
	defer func() {
		errRemove := os.RemoveAll(dir)
		if errRemove != nil {
			protocol.Err("Could not remove directory", errRemove)
		}
	}()
	a.NoError(errTempDir)

	args := Args{
		Listen:      "localhost:0",
		KVBackend:   "memory",
		MSBackend:   "file",
		StoragePath: dir,
		GcmEnable:   true,
		GcmApiKey:   "WILL BE OVERWRITTEN",
		GcmWorkers:  nWorkers}

	service := StartService(args)

	protocol.Debug("Overwriting the GCM Sender with a Mock")
	gcmConnector, ok := service.Modules()[4].(*gcm.Connector)
	a.True(ok, "Modules[4] should be of type GCMConnector")
	gcmConnector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithJsonResponse(http.StatusOK, testutil.CorrectGcmResponseMessageJSON, nil))

	gomaxprocs := runtime.GOMAXPROCS(0)
	clients := make([]client.Client, 0, gomaxprocs)
	for clientID := 0; clientID < gomaxprocs; clientID++ {
		location := "ws://" + service.WebServer().GetAddr() + "/stream/user/" + strconv.Itoa(clientID)
		client, err := client.Open(location, "http://localhost/", 1000, true)
		a.NoError(err)
		clients = append(clients, client)
	}

	// create a topic
	url := fmt.Sprintf("http://%s/gcm/0/gcmId0/subscribe/topic", service.WebServer().GetAddr())
	response, errPost := http.Post(url, "text/plain", bytes.NewBufferString(""))
	a.NoError(errPost)
	a.Equal(response.StatusCode, 200)
	body, errReadAll := ioutil.ReadAll(response.Body)
	a.NoError(errReadAll)
	a.Equal("registered: /topic\n", string(body))

	protocol.Debug("starting the benchmark timer")
	start := time.Now()
	b.ResetTimer()

	protocol.Debug("sending multiple messages from each client in separate goroutines")
	var wg sync.WaitGroup
	wg.Add(gomaxprocs)
	for _, c := range clients {
		go func(c client.Client) {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				a.NoError(sampleSend(c))
			}
		}(c)
	}
	wg.Wait()

	// stop service (and wait for all the messages to be processed during the given grace period)
	err := service.Stop()
	a.Nil(err)

	end := time.Now()
	b.StopTimer()

	return float64(b.N*gomaxprocs) / end.Sub(start).Seconds()
}

func sendBroadcastSample(c client.Client) error {
	return c.Send("/gcm/broadcast", "general offer", "{id:id}")
}

func sendMessageSample(c client.Client) error {
	return c.Send("topic", "personalized offer", "{id:id}")
}
