package redundancy

import (
	"bytes"
	"fmt"
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/gubled"
	"github.com/smancke/guble/gubled/config"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	gcmTopic = "/topic"
)

type param struct {
	timeout   time.Duration // gcm timeout response
	receiveC  chan bool
	doneC     chan struct{}
	connector *gcm.Connector
	service   *server.Service
	sent      int // sent messages
	received  int // received messages
	dir       string
}

func subscribeToServiceNode(t *testing.T, service *server.Service) {
	urlFormat := fmt.Sprintf("http://%s/gcm/%%d/gcmId%%d/subscribe/%%s", service.WebServer().GetAddr())

	a := assert.New(t)

	response, errPost := http.Post(
		fmt.Sprintf(urlFormat, 1, 1, strings.TrimPrefix(gcmTopic, "/")), "text/plain", bytes.NewBufferString(""),
	)
	a.NoError(errPost)
	a.Equal(response.StatusCode, 200)

	body, errReadAll := ioutil.ReadAll(response.Body)
	a.NoError(errReadAll)
	a.Equal("registered: /topic\n", string(body))

}

func checkForGcmSubscribe(t *testing.T, p *param, expectedCount int) {
	logger.Info("mamaa masssiii")

	a := assert.New(t)
	go func() {
		for {
			select {
			case <-p.receiveC:
				p.received++
				logger.WithField("received", p.received).Info("Received gcm call")
			case <-p.doneC:
				return
			}
		}

		a.Equal(p.received, expectedCount)

	}()

}

func startService(t *testing.T, listenPort, dirName string, nodeID, nodePort int, useTempFolder bool) *param {
	receiveC := make(chan bool)
	doneC := make(chan struct{})
	timeout := time.Millisecond * 100
	a := assert.New(t)

	var dir string
	if useTempFolder {
		dir, errTempDir := ioutil.TempDir("", dirName)
		defer func() {
			errRemove := os.RemoveAll(dir)
			if errRemove != nil {
				logger.WithError(errRemove).WithField("module", "testing").Error("Could not remove directory")
			}
		}()
		a.NoError(errTempDir)
	} else {
		dir = "/tmp/" + dirName
		errTempDir := os.MkdirAll(dir, 0777)
		a.NoError(errTempDir)
		logger.Warn("Directory should be deleted manually")
	}

	*config.HttpListen = "localhost:" + listenPort
	*config.KVS = "file"
	*config.MS = "file"
	*config.StoragePath = dir

	*config.GCM.Enabled = true
	*config.GCM.APIKey = "WILL BE OVERWRITTEN"
	*config.GCM.Workers = 4

	*config.Cluster.NodeID = nodeID
	*config.Cluster.NodePort = nodePort
	addr, err := net.ResolveTCPAddr("", "127.0.0.1:10000")
	a.NoError(err)

	*config.Cluster.Remotes = []*net.TCPAddr{addr}

	service := gubled.StartService()

	var gcmConnector *gcm.Connector
	var ok bool
	for _, iface := range service.ModulesSortedByStartOrder() {
		gcmConnector, ok = iface.(*gcm.Connector)
		if ok {
			break
		}
	}
	a.True(ok, "There should be a module of type GCMConnector")

	gcmConnector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithCountAndTimeout(http.StatusOK, testutil.SuccessGCMResponse, receiveC, timeout))

	return &param{
		timeout:   timeout,
		receiveC:  receiveC,
		doneC:     doneC,
		connector: gcmConnector,
		service:   service,
		dir:       dir,
	}

}

func Test_Subscribe_on_random_node(t *testing.T) {

	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	firstServiceParam := startService(t, "8080", "dir1", 1, 10000, true)
	a.NotNil(firstServiceParam)

	secondServiceParam := startService(t, "8081", "dir2", 2, 10001, true)
	a.NotNil(secondServiceParam)
	//subscribe on first node
	subscribeToServiceNode(t, firstServiceParam.service)

	//connect a clinet and send a message
	client1, err1 := client.Open("ws://"+firstServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
	a.NoError(err1)

	err := client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	//only one message should be received but only on the first node.Every message should be delivered only once.
	checkForGcmSubscribe(t, firstServiceParam, 1)
	checkForGcmSubscribe(t, secondServiceParam, 0)

	subscribeToServiceNode(t, secondServiceParam.service)
}

func Test_Subscribe_working_After_Node_Restart(t *testing.T) {

	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	firstServiceParam := startService(t, "8080", "dir1", 1, 10000, false)
	a.NotNil(firstServiceParam)

	secondServiceParam := startService(t, "8081", "dir2", 2, 10001, false)
	a.NotNil(secondServiceParam)
	//subscribe on first node
	subscribeToServiceNode(t, firstServiceParam.service)

	//connect a clinet and send a message
	client1, err1 := client.Open("ws://"+firstServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
	a.NoError(err1)

	err := client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	//only one message should be received but only on the first node.Every message should be delivered only once.
	checkForGcmSubscribe(t, firstServiceParam, 1)
	checkForGcmSubscribe(t, secondServiceParam, 0)

	// stop a node
	err = firstServiceParam.service.Stop()
	a.NoError(err)
	time.Sleep(time.Millisecond * 150)

	// restart the service
	restartedServiceParam := startService(t, "8080", "dir1", 1, 10000, false)
	a.NotNil(restartedServiceParam)

	//   send a message to the former subscription.
	client1, err1 = client.Open("ws://"+restartedServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
	a.NoError(err1)

	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err, "Subscription should work even after node restart")

	//only one message should be received but only on the first node.Every message should be delivered only once.
	time.Sleep(time.Millisecond * 150)
	checkForGcmSubscribe(t, restartedServiceParam, 1)
	checkForGcmSubscribe(t, secondServiceParam, 0)

	//clean up

	errRemove := os.RemoveAll(restartedServiceParam.dir)
	a.NoError(errRemove)
	errRemove = os.RemoveAll(secondServiceParam.dir)
	a.NoError(errRemove)
}

func Test_independent_Receiveing(t *testing.T) {

	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	firstServiceParam := startService(t, "8080", "dir1", 1, 10000, true)
	a.NotNil(firstServiceParam)

	secondServiceParam := startService(t, "8081", "dir2", 2, 10001, true)
	a.NotNil(secondServiceParam)
	//subscribe on first node
	subscribeToServiceNode(t, firstServiceParam.service)

	//connect a clinet and send a message
	client1, err1 := client.Open("ws://"+firstServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
	a.NoError(err1)

	err := client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	//only one message should be received but only on the first node.Every message should be delivered only once.
	checkForGcmSubscribe(t, firstServiceParam, 1)
	checkForGcmSubscribe(t, secondServiceParam, 0)

	//subscribeToServiceNode(t, secondServiceParam.service)

	//NOW connect to second node
	client1, err1 = client.Open("ws://"+secondServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8081", 1000, true)
	a.NoError(err1)

	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	//only one message should be received but only on the first node.Every message should be delivered only once.
	checkForGcmSubscribe(t, firstServiceParam, 0)
	checkForGcmSubscribe(t, secondServiceParam, 1)

}
