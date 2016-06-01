package gubled

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"

	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func BenchmarkGCMConnector_BroadcastMessagesSingleWorker(b *testing.B) {
	throughput := throughputBroadcastMessages(b, 1)
	protocol.Info("Throughput for single worker: %8f msg/sec\n", throughput)
}

func BenchmarkGCMConnector_BroadcastMessagesMultipleWorkers(b *testing.B) {
	gomaxprocs := runtime.GOMAXPROCS(0)
	throughput := throughputBroadcastMessages(b, gomaxprocs)
	protocol.Info("Throughput for GOMAXPROCS (%v): %8f msg/sec\n", gomaxprocs, throughput)
}

func throughputBroadcastMessages(b *testing.B, nWorkers int) float64 {
	testutil.EnableDebugForMethod()
	protocol.Debug("b.N=%v\n", b.N)
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(b)

	dir, errTempDir := ioutil.TempDir("", "guble_benchmark_gcm_test")
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
	gcmConnector, ok := service.Modules[2].(*gcm.GCMConnector)
	a.True(ok, "Modules[2] should be of type GCMConnector")
	gcmConnector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithJsonResponse(http.StatusOK, testutil.CorrectGcmResponseMessageJSON, nil))

	service.Start()

	gomaxprocs := runtime.GOMAXPROCS(0)
	clients := make([]client.Client, 0, gomaxprocs)
	for clientID := 0; clientID < gomaxprocs; clientID++ {
		location := "ws://" + service.WebServer().GetAddr() + "/stream/user/" + strconv.Itoa(clientID)
		client, err := client.Open(location, "http://localhost/", 1000, true)
		a.NoError(err)
		clients = append(clients, client)
	}

	start := time.Now()
	b.ResetTimer()

	for clientID, c := range clients {
		go func(cID int, c client.Client) {
			for i := 0; i < b.N; i++ {
				protocol.Debug("send %d %d", cID, i)
				a.NoError(c.Send("/gcm/broadcast", "special offer", "{id:id}"))
			}
		}(clientID, c)
	}

	//TODO Cosmin: this delay should be eliminated after clean-shutdown is operational
	time.Sleep(2 * time.Second)

	// stop service (and wait for all the messages to be processed)
	service.StopGracePeriod = 10 * time.Second
	err := service.Stop()
	a.Nil(err)

	end := time.Now()
	b.StopTimer()

	return float64(b.N*gomaxprocs) / end.Sub(start).Seconds()
}
