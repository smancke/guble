package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/smancke/guble/server/gcm"
	"github.com/smancke/guble/testutil"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/service"
	"github.com/stretchr/testify/assert"
	"gopkg.in/alecthomas/kingpin.v2"
)

type TestNodeConfig struct {
	HttpListen  string // "host:port" format or just ":port"
	NodeID      int
	NodePort    int
	StoragePath string // if empty it will create a temporary directory
	MemoryStore string
	KVStore     string
	Remotes     []string
}

func (tnc *TestNodeConfig) parseConfig() error {
	var err error

	dir := tnc.StoragePath
	if dir == "" {
		dir, err = ioutil.TempDir("", "guble_test")
		if err != nil {
			return err
		}
	}
	tnc.StoragePath = dir

	args := []string{
		"--log", "debug",
		"--http", tnc.HttpListen,
		"--storage-path", tnc.StoragePath,
		"--health-endpoint", "",

		"--gcm",
		"--gcm-api-key", "WILL BE OVERWRITTEN",
		"--gcm-workers", "4",
	}

	if tnc.MemoryStore != "" {
		args = append(args, "--ms", tnc.MemoryStore)
	}

	if tnc.KVStore != "" {
		args = append(args, "--kvs", tnc.KVStore)
	}

	if tnc.NodeID > 0 {
		args = append(
			args,
			"--node-id", strconv.Itoa(tnc.NodeID),
			"--node-port", strconv.Itoa(tnc.NodePort),
		)
		args = append(args, tnc.Remotes...)
	}

	_, err = kingpin.CommandLine.Parse(args)
	return err
}

type TestClusterNode struct {
	TestNodeConfig
	t       *testing.T
	GCM     *TestGCM
	Service *service.Service
}

func NewTestClusterNode(t *testing.T, nodeConfig TestNodeConfig) *TestClusterNode {
	a := assert.New(t)

	err := nodeConfig.parseConfig()
	if !a.NoError(err) {
		return nil
	}

	service := StartService()

	var (
		gcmConnector *gcm.Connector
		ok           bool
	)
	for _, iface := range service.ModulesSortedByStartOrder() {
		if gcmConnector, ok = iface.(*gcm.Connector); ok {
			break
		}
	}
	if !a.True(ok, "There should be a module of type GCMConnector") {
		return nil
	}

	return &TestClusterNode{
		TestNodeConfig: nodeConfig,
		t:              t,
		GCM: &TestGCM{
			t:         t,
			Connector: gcmConnector,
		},
		Service: service,
	}
}

func (tcn *TestClusterNode) Client(userID string, bufferSize int, autoReconnect bool) (client.Client, error) {
	serverAddr := tcn.Service.WebServer().GetAddr()
	wsURL := "ws://" + serverAddr + "/stream/user/" + userID
	httpURL := "http://" + serverAddr

	return client.Open(wsURL, httpURL, bufferSize, autoReconnect)
}

func (tcn *TestClusterNode) Subscribe(topic string) {
	tcn.GCM.subscribe(tcn.Service.WebServer().GetAddr(), topic)
}

func (tcn *TestClusterNode) Cleanup() {
	tcn.GCM.Cleanup()
	err := tcn.Service.Stop()
	assert.NoError(tcn.t, err)

	err = os.RemoveAll(tcn.StoragePath)
	assert.NoError(tcn.t, err)
}

type TestGCM struct {
	t         *testing.T
	Connector *gcm.Connector
	timeout   time.Duration
	Received  int // received messages
	receiveC  chan bool
	stopC     chan struct{}
}

func (tgcm *TestGCM) SetupRoundTripper(timeout time.Duration, bufferSize int, response string) {
	tgcm.receiveC = make(chan bool, bufferSize)
	tgcm.timeout = timeout
	tgcm.Connector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithCountAndTimeout(http.StatusOK, response, tgcm.receiveC, timeout))
}

func (tgcm *TestGCM) subscribe(addr, topic string) {
	urlFormat := fmt.Sprintf("http://%s/gcm/%%d/gcmId%%d/subscribe/%%s", addr)

	a := assert.New(tgcm.t)

	response, err := http.Post(
		fmt.Sprintf(urlFormat, 1, 1, strings.TrimPrefix(topic, "/")), "text/plain", bytes.NewBufferString(""),
	)
	if a.NoError(err) {
		a.Equal(response.StatusCode, 200)
	}

	body, err := ioutil.ReadAll(response.Body)
	a.NoError(err)
	a.Equal("registered: /topic\n", string(body))
}

// Wait waits count * tgcm.timeout, wait ensure count number of messages have been waited to pass
// through GCM round tripper
func (tgcm *TestGCM) Wait(count int) {
	time.Sleep(time.Duration(count) * tgcm.timeout)
}

// Receive starts a goroutine that will receive on the receiveC and increment the Received counter
// Returns an error if channel is not create
// func (tgcm *TestGCM) Receive() error {
// 	if tgcm.receiveC == nil {
// 		return fmt.Errorf("Round tripper not created")
// 	}

// 	go func() {
// 		for {
// 			if _, opened := <-tgcm.receiveC; opened {
// 				tgcm.Lock()
// 				tgcm.Received++
// 				tgcm.Unlock()
// 			}
// 		}
// 	}()

// 	return nil
// }

func (tgcm *TestGCM) CheckReceived(expected int, to time.Duration) {
	for {
		select {
		case _, opened := <-tgcm.receiveC:
			if !opened {
				break
			}
			tgcm.Received++
		case <-time.After(to):
			assert.Fail(tgcm.t, "Failed to receive GCM message before timeout")
		}
	}

	assert.Equal(tgcm.t, expected, tgcm.Received)
}

func (tgcm *TestGCM) Reset() {
	tgcm.Received = 0
}

func (tgcm *TestGCM) Cleanup() {
	if tgcm.receiveC != nil {
		close(tgcm.receiveC)
	}
}
