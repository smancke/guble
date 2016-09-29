package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/smancke/guble/server/fcm"
	"github.com/smancke/guble/testutil"

	"errors"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/service"
	"github.com/stretchr/testify/assert"
	"gopkg.in/alecthomas/kingpin.v2"
)

type testClusterNodeConfig struct {
	HttpListen  string // "host:port" format or just ":port"
	NodeID      int
	NodePort    int
	StoragePath string // if empty it will create a temporary directory
	MemoryStore string
	KVStore     string
	Remotes     string
}

func (tnc *testClusterNodeConfig) parseConfig() error {
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

		"--fcm",
		"--fcm-api-key", "WILL BE OVERWRITTEN",
		"--fcm-workers", "4",
	}

	if tnc.MemoryStore != "" {
		args = append(args, "--ms", tnc.MemoryStore)
	}

	if tnc.KVStore != "" {
		args = append(args, "--kvs", tnc.KVStore)
	}

	if tnc.NodeID > 0 {
		if tnc.Remotes == "" {
			return fmt.Errorf("Missing Remotes value when running in cluster mode.")
		}

		args = append(
			args,
			"--node-id", strconv.Itoa(tnc.NodeID),
			"--node-port", strconv.Itoa(tnc.NodePort),
			"--remotes", tnc.Remotes,
		)
	}

	_, err = kingpin.CommandLine.Parse(args)
	return err
}

type testClusterNode struct {
	testClusterNodeConfig
	t       *testing.T
	FCM     *TestFCM
	Service *service.Service
}

func newTestClusterNode(t *testing.T, nodeConfig testClusterNodeConfig) *testClusterNode {
	a := assert.New(t)

	err := nodeConfig.parseConfig()
	if !a.NoError(err) {
		return nil
	}

	s := StartService()

	var (
		gcmConnector *fcm.Connector
		ok           bool
	)
	for _, iface := range s.ModulesSortedByStartOrder() {
		if gcmConnector, ok = iface.(*fcm.Connector); ok {
			break
		}
	}
	if !a.True(ok, "There should be a module of type GCMConnector") {
		return nil
	}

	return &testClusterNode{
		testClusterNodeConfig: nodeConfig,
		t: t,
		FCM: &TestFCM{
			t:         t,
			Connector: gcmConnector,
		},
		Service: s,
	}
}

func (tcn *testClusterNode) client(userID string, bufferSize int, autoReconnect bool) (client.Client, error) {
	serverAddr := tcn.Service.WebServer().GetAddr()
	wsURL := "ws://" + serverAddr + "/stream/user/" + userID
	httpURL := "http://" + serverAddr

	return client.Open(wsURL, httpURL, bufferSize, autoReconnect)
}

func (tcn *testClusterNode) Subscribe(topic, id string) {
	tcn.FCM.subscribe(tcn.Service.WebServer().GetAddr(), topic, id)
}

func (tcn *testClusterNode) Unsubscribe(topic, id string) {
	tcn.FCM.unsubscribe(tcn.Service.WebServer().GetAddr(), topic, id)
}

func (tcn *testClusterNode) cleanup(removeDir bool) {
	tcn.FCM.cleanup()
	err := tcn.Service.Stop()
	assert.NoError(tcn.t, err)

	if removeDir {
		err = os.RemoveAll(tcn.StoragePath)
		assert.NoError(tcn.t, err)
	}
}

type TestFCM struct {
	sync.RWMutex
	t         *testing.T
	Connector *fcm.Connector
	Received  int // received messages
	receiveC  chan bool
	timeout   time.Duration
}

func (tgcm *TestFCM) setupRoundTripper(timeout time.Duration, bufferSize int, response string) {
	tgcm.receiveC = make(chan bool, bufferSize)
	tgcm.timeout = timeout
	tgcm.Connector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithCountAndTimeout(http.StatusOK, response, tgcm.receiveC, timeout))

	// start counting the received messages to FCM
	tgcm.receive()
}

func (tgcm *TestFCM) subscribe(addr, topic, id string) {
	urlFormat := fmt.Sprintf("http://%s/gcm/user_%%s/gcm_%%s/subscribe/%%s", addr)

	a := assert.New(tgcm.t)

	response, err := http.Post(
		fmt.Sprintf(urlFormat, id, id, strings.TrimPrefix(topic, "/")), "text/plain", bytes.NewBufferString(""),
	)
	if a.NoError(err) {
		a.Equal(response.StatusCode, 200)
	}

	body, err := ioutil.ReadAll(response.Body)
	a.NoError(err)
	a.Equal(fmt.Sprintf(`{"subscribed":"%s"}`, topic), string(body))
}

func (tgcm *TestFCM) unsubscribe(addr, topic, id string) {
	urlFormat := fmt.Sprintf("http://%s/gcm/user_%%s/gcm_%%s/subscribe/%%s", addr)

	a := assert.New(tgcm.t)

	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf(urlFormat, id, id, strings.TrimPrefix(topic, "/")),
		bytes.NewBufferString(""))
	a.NoError(err)

	hc := &http.Client{}

	response, err := hc.Do(req)
	if a.NoError(err) {
		a.Equal(response.StatusCode, 200)
	}

	body, err := ioutil.ReadAll(response.Body)
	a.NoError(err)
	a.Equal(fmt.Sprintf(`{"unsubscribed":"%s"}`, topic), string(body))
}

// Wait waits count * tgcm.timeout, wait ensure count number of messages have been waited to pass
// through GCM round tripper
func (tgcm *TestFCM) wait(count int) {
	time.Sleep(time.Duration(count) * tgcm.timeout)
}

// Receive starts a goroutine that will receive on the receiveC and increment the Received counter
// Returns an error if channel is not create
func (tgcm *TestFCM) receive() error {
	if tgcm.receiveC == nil {
		return errors.New("Round tripper not created")
	}

	go func() {
		for {
			if _, opened := <-tgcm.receiveC; opened {
				tgcm.Lock()
				tgcm.Received++
				tgcm.Unlock()
			}
		}
	}()

	return nil
}

func (tgcm *TestFCM) checkReceived(expected int) {
	time.Sleep((50 * time.Millisecond) + tgcm.timeout)
	tgcm.RLock()
	defer tgcm.RUnlock()
	assert.Equal(tgcm.t, expected, tgcm.Received)
}

func (tgcm *TestFCM) reset() {
	tgcm.Lock()
	defer tgcm.Unlock()
	tgcm.Received = 0
}

func (tgcm *TestFCM) cleanup() {
	if tgcm.receiveC != nil {
		close(tgcm.receiveC)
	}
}
