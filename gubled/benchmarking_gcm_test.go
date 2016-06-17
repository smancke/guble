package gubled

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/gubled/config"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/testutil"

	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"testing"

	log "github.com/Sirupsen/logrus"
)

type sender func(c client.Client) error

type benchParams struct {
	workers     int
	subCount    int
	gcmTimeout  time.Duration
	sender      sender
	clientCount int
}

var service *server.Service

// func BenchmarkGCMConnector_BroadcastMessagesSingleWorker(b *testing.B) {
// 	throughput := throughputSend(b, 1, sendBroadcastSample)
// 	fmt.Printf("Broadcast throughput for single worker: %.2f msg/sec\n", throughput)
// }

// func BenchmarkGCMConnector_BroadcastMessagesMultipleWorkers(b *testing.B) {
// 	gomaxprocs := runtime.GOMAXPROCS(0)
// 	throughput := throughputSend(b, gomaxprocs, sendBroadcastSample)
// 	fmt.Printf("Broadcast throughput for GOMAXPROCS (%v): %.2f msg/sec\n", gomaxprocs, throughput)
// }

func BenchmarkGCMConnector_SendMessagesSingleWorker(b *testing.B) {
	throughput := throughputSend(b, 1, sendMessageSample)
	fmt.Printf("Send throughput for single worker: %.2f msg/sec\n", throughput)
}

func BenchmarkGCMConnector_SendMessagesMultipleWorkers(b *testing.B) {
	gomaxprocs := runtime.GOMAXPROCS(0)
	throughput := throughputSend(b, gomaxprocs, sendMessageSample)
	fmt.Printf("Send throughput for GOMAXPROCS (%v): %.2f msg/sec\n", gomaxprocs, throughput)
}

func throughputSend(b *testing.B, params benchParams) float64 {
	// defer testutil.EnableDebugForMethod()()
	defer testutil.ResetDefaultRegistryHealthCheck()
	countC := setUp(b, nWorkers, gcmTimeout)

	a := assert.New(b)

	gomaxprocs := runtime.GOMAXPROCS(0)
	wsURL := "ws://" + service.WebServer().GetAddr() + "/stream/user/"

	clients := make([]client.Client, 0, gomaxprocs)
	for clientID := 0; clientID < gomaxprocs; clientID++ {
		location := wsURL + strconv.Itoa(clientID)
		client, err := client.Open(location, "http://localhost/", 1000, true)
		a.NoError(err)
		clients = append(clients, client)
	}

	// Report allocations also
	b.ReportAllocs()

	// Reset timer to start the actual timing
	b.ResetTimer()
	start := time.Now()

	var wg sync.WaitGroup
	doneC := make(chan struct{})

	// read count
	go func() {
		counter := 0
		for {
			select {
			case <-countC:
				counter++
				wg.Done()
				log.WithField("count", counter).Info("Passed messages")
			case <-doneC:
				return
			}
		}
	}()

	// wait until all messages are sent
	log.WithFields(log.Fields{
		"b.N":       b.N,
		"waitCount": b.N * len(clients) * nWorkers,
	}).Debug("Bench wait group")
	wg.Add(b.N * len(clients) * nWorkers)
	for _, c := range clients {
		go func(c client.Client) {
			for i := 0; i < b.N; i++ {
				a.NoError(sampleSend(c))
			}
		}(c)
	}
	wg.Wait()
	close(doneC)

	// stop timer after the actual test
	b.StopTimer()
	end := time.Now()

	// stop service (and wait for all the messages to be processed during the given grace period)
	tearDown(b)

	return float64(b.N*gomaxprocs) / end.Sub(start).Seconds()
}

// func sendBroadcastSample(c client.Client) error {
// 	return c.Send("/gcm/broadcast", "general offer", "{id:id}")
// }

func sendMessageSample(c client.Client) error {
	return c.Send("/topic", "personalized offer", "{id:id}")
}

// start the service
func setUp(b *testing.B, nWorkers int) chan int {
	a := assert.New(b)

	dir, errTempDir := ioutil.TempDir("", "guble_benchmarking_gcm_test")
	defer func() {
		errRemove := os.RemoveAll(dir)
		if errRemove != nil {
			log.WithFields(log.Fields{"module": "testing", "err": errRemove}).Error("Could not remove directory")
		}
	}()
	a.NoError(errTempDir)

	*config.Listen = "localhost:0"
	*config.KVBackend = "memory"
	*config.MSBackend = "file"
	*config.StoragePath = dir
	*config.GCM.Enabled = true
	*config.GCM.APIKey = "WILL BE OVERWRITTEN"
	*config.GCM.Workers = nWorkers

	service = StartService()

	gcmConnector, ok := service.Modules()[4].(*gcm.Connector)
	a.True(ok, "Modules[4] should be of type GCMConnector")

	countC := make(chan int)

	gcmConnector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithCount(http.StatusOK, testutil.SuccessGCMResponse, countC))

	urlFormat := fmt.Sprintf("http://%s/gcm/%%d/gcmId%%d/subscribe/topic", service.WebServer().GetAddr())
	for i := 1; i <= nWorkers; i++ {
		// create GCM subscription with topic: /topic
		response, errPost := http.Post(fmt.Sprintf(urlFormat, i, i), "text/plain", bytes.NewBufferString(""))
		a.NoError(errPost)
		a.Equal(response.StatusCode, 200)

		body, errReadAll := ioutil.ReadAll(response.Body)
		a.NoError(errReadAll)
		a.Equal("registered: /topic\n", string(body))

	}

	return countC
}

func tearDown(b *testing.B) {
	assert.NoError(b, service.Stop())
	service = nil
}
