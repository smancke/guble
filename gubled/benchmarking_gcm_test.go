package gubled

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/testutil"

	"github.com/stretchr/testify/assert"

	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"
)

func BenchmarkGCMConnector_BroadcastMessagesSingleWorker(b *testing.B) {
	throughput := throughputBroadcastMessages(b, 1)
	fmt.Printf("Throughput for single worker: %8f msg/sec\n", throughput)
}

func BenchmarkGCMConnector_BroadcastMessagesMaxWorkers(b *testing.B) {
	gomaxprocs := runtime.GOMAXPROCS(0)
	throughput := throughputBroadcastMessages(b, gomaxprocs)
	fmt.Printf("Throughput for GOMAXPROCS (%v): %8f msg/sec\n", gomaxprocs, throughput)
}

func throughputBroadcastMessages(b *testing.B, nWorkers int) float64 {
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(b)

	dir, _ := ioutil.TempDir("", "guble_benchmark_test")
	defer os.RemoveAll(dir)

	args := Args{
		Listen:      "localhost:0",
		KVBackend:   "memory",
		MSBackend:   "file",
		StoragePath: dir,
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

	fmt.Printf("N=%v\n", b.N)
	location := "http://" + service.WebServer().GetAddr() + "/gcm/broadcast"

	start := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.Open(location, "http://localhost/", 1000, true)
		a.NoError(err)
	}

	end := time.Now()
	b.StopTimer()

	// wait for the messages to be processed by http server
	time.AfterFunc(500*time.Millisecond, func() {
		err := service.Stop()
		a.Nil(err)
	})

	return float64(b.N) / end.Sub(start).Seconds()
}
