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

	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/server/fcm"

	"errors"

	"github.com/cosminrentea/gobbler/client"
	"github.com/cosminrentea/gobbler/server/service"
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
		"--ws",
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
		fcmConnector connector.ResponsiveConnector
		ok           bool
	)
	for _, iface := range s.ModulesSortedByStartOrder() {
		if fcmConnector, ok = iface.(connector.ResponsiveConnector); ok {
			break
		}
	}
	if !a.True(ok, "There should be a module of type ResponsiveConnector (FCM)") {
		return nil
	}

	return &testClusterNode{
		testClusterNodeConfig: nodeConfig,
		t: t,
		FCM: &TestFCM{
			t:         t,
			Connector: fcmConnector,
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
	Connector connector.ResponsiveConnector
	Received  int // received messages
	receiveC  chan bool
	timeout   time.Duration
}

func (tfcm *TestFCM) setupRoundTripper(timeout time.Duration, bufferSize int, response string) {
	tfcm.receiveC = make(chan bool, bufferSize)
	tfcm.timeout = timeout
	sender, err := fcm.CreateFcmSender(response, tfcm.receiveC, timeout)
	assert.NoError(tfcm.t, err)
	tfcm.Connector.SetSender(sender)
	// start counting the received messages to FCM
	tfcm.receive()
}

func (tfcm *TestFCM) subscribe(addr, topic, id string) {
	urlFormat := fmt.Sprintf("http://%s/fcm/user_%%s/gcm_%%s/%%s", addr)

	a := assert.New(tfcm.t)

	response, err := http.Post(
		fmt.Sprintf(urlFormat, id, id, strings.TrimPrefix(topic, "/")), "text/plain", bytes.NewBufferString(""),
	)
	if a.NoError(err) {
		a.Equal(response.StatusCode, 200)
	}

	body, err := ioutil.ReadAll(response.Body)
	a.NoError(err)
	a.Equal(fmt.Sprintf("{\"subscribed\":\"%s\"}", topic), string(body))
}

func (tfcm *TestFCM) unsubscribe(addr, topic, id string) {
	urlFormat := fmt.Sprintf("http://%s/fcm/user_%%s/gcm_%%s/%%s", addr)

	a := assert.New(tfcm.t)

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
func (tfcm *TestFCM) wait(count int) {
	time.Sleep(time.Duration(count) * tfcm.timeout)
}

// Receive starts a goroutine that will receive on the receiveC and increment the Received counter
// Returns an error if channel is not create
func (tfcm *TestFCM) receive() error {
	if tfcm.receiveC == nil {
		return errors.New("Round tripper not created")
	}

	go func() {
		for {
			if _, opened := <-tfcm.receiveC; opened {
				tfcm.Lock()
				tfcm.Received++
				tfcm.Unlock()
			}
		}
	}()

	return nil
}

func (tfcm *TestFCM) checkReceived(expected int) {
	time.Sleep((50 * time.Millisecond) + tfcm.timeout)
	tfcm.RLock()
	defer tfcm.RUnlock()
	assert.Equal(tfcm.t, expected, tfcm.Received)
}

func (tfcm *TestFCM) reset() {
	tfcm.Lock()
	defer tfcm.Unlock()
	tfcm.Received = 0
}

func (tfcm *TestFCM) cleanup() {
	if tfcm.receiveC != nil {
		close(tfcm.receiveC)
	}
}
