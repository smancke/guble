package gubled

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"

	"github.com/stretchr/testify/assert"

	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func BenchmarkGCMConnector_BroadcastMessagesSingleWorker(b *testing.B) {
	throughput := throughputBroadcastMessages(b, 1)
	fmt.Printf("Throughput for single worker: %8f msg/sec\n", throughput)
}

func BenchmarkGCMConnector_BroadcastMessagesMultipleWorkers(b *testing.B) {
	gomaxprocs := runtime.GOMAXPROCS(0)
	throughput := throughputBroadcastMessages(b, gomaxprocs)
	fmt.Printf("Throughput for GOMAXPROCS (%v): %8f msg/sec\n", gomaxprocs, throughput)
}

func throughputBroadcastMessages(b *testing.B, nWorkers int) float64 {
	testutil.EnableDebugForMethod()
	protocol.Debug("b.N=%v\n", b.N)
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(b)

	dirName := "/tmp/BenchmarkGCMConnector_BroadcastMessagesMultipleWorkers"
	err := os.Mkdir(dirName, os.ModeDir|os.ModePerm)
	defer func() {
		err = os.RemoveAll(dirName)
		if err != nil {
			protocol.Err("Could not remove directory", err)
		}
	}()

	args := Args{
		Listen:      "localhost:0",
		KVBackend:   "memory",
		MSBackend:   "file",
		StoragePath: dirName,
		GcmEnable:   true,
		GcmApiKey:   "WILL BE OVERWRITTEN",
		GcmWorkers:  nWorkers}

	service := StartService(args)

	for _, module := range CreateModules(*service.Router(), args) {
		if gcmConnector, ok := module.(gcm.GCMConnector); ok {
			gcmConnector.Sender = testutil.CreateGcmSender(
				testutil.CreateRoundTripperWithJsonResponse(http.StatusOK, testutil.CorrectGcmResponseMessageJSON, nil))
		}
	}

	service.Start()

	gomaxprocs := runtime.GOMAXPROCS(0)
	clients := make([]client.Client, gomaxprocs)
	for cId := 0; cId < gomaxprocs; cId++ {
		location := "ws://" + service.WebServer().GetAddr() + "/stream/user" + strconv.Itoa(cId)
		client, err := client.Open(location, "http://localhost/", 1000, true)
		a.NoError(err)
		clients[cId] = client
	}

	start := time.Now()
	b.ResetTimer()

	for _, client := range clients {
		go func() {
			for i := 0; i < b.N; i++ {
				client.Send("/gcm/broadcast", "special offer", "{id:id}")
			}
		}()
	}

	// wait for all the messages to be processed
	time.Sleep(5 * time.Second)
	service.StopGracePeriod = 10 * time.Second
	err = service.Stop()
	a.Nil(err)

	end := time.Now()
	b.StopTimer()

	return float64(b.N*gomaxprocs) / end.Sub(start).Seconds()
}
