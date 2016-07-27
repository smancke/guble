package server

import (
	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/gcm"
	"github.com/smancke/guble/server/service"
	"github.com/smancke/guble/testutil"

	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type param struct {
	timeout   time.Duration // gcm timeout response
	receiveC  chan bool
	connector *gcm.Connector
	service   *service.Service
	sent      int // sent messages
	received  int // received messages
	dir       string
}

func subscribeToServiceNode(t *testing.T, service *service.Service) {
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

func checkNumberOfRcvMsgOnGCM(t *testing.T, p *param, expectedCount int) {
	a := assert.New(t)

	for {
		select {
		case <-p.receiveC:
			p.received++
			logger.WithField("received", p.received).Info("++Received gcm call")
		case <-time.After(time.Second * 2):
			a.Equal(p.received, expectedCount)
			return
		}
	}
	logger.WithFields(log.Fields{
		"cond":           p.received == expectedCount,
		"p.rece":         p.received,
		"expectedCpount": expectedCount,
	}).Info("++Out")

	a.Equal(expectedCount, p.received)
}

func startService(t *testing.T, listenPort, dirName string, nodeID, nodePort int, useTempFolder bool) *param {
	receiveC := make(chan bool, 10)

	timeout := time.Millisecond * 100
	a := assert.New(t)

	var dir string
	if useTempFolder {
		var errTempDir error
		dir, errTempDir = ioutil.TempDir("", dirName)
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

	service := StartService()

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
		connector: gcmConnector,
		service:   service,
		dir:       dir,
	}

}

func Test_Subscribe_on_random_node(t *testing.T) {
	testutil.SkipIfShort(t)

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
	checkNumberOfRcvMsgOnGCM(t, firstServiceParam, 1)
	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 0)

	subscribeToServiceNode(t, secondServiceParam.service)

	//TODO MARIAN  add stop
	defer func() {
		err = firstServiceParam.service.Stop()
		a.NoError(err)

		err = secondServiceParam.service.Stop()
		a.NoError(err)

		errRemove := os.RemoveAll(firstServiceParam.dir)
		a.NoError(errRemove)

		errRemove = os.RemoveAll(secondServiceParam.dir)
		a.NoError(errRemove)
	}()
}

func Test_Subscribe_working_After_Node_Restart(t *testing.T) {
	//TODO MARIAN   removed this after stop is working
	testutil.SkipIfShort(t)
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
	checkNumberOfRcvMsgOnGCM(t, firstServiceParam, 1)
	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 0)

	// stop a node
	err = firstServiceParam.service.Stop()
	a.NoError(err)
	time.Sleep(time.Millisecond * 150)

	// // restart the service
	restartedServiceParam := startService(t, "8080", "dir1", 1, 10000, false)
	a.NotNil(restartedServiceParam)

	//   send a message to the former subscription.
	client1, err1 = client.Open("ws://"+restartedServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
	a.NoError(err1)

	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err, "Subscription should work even after node restart")

	//only one message should be received but only on the first node.Every message should be delivered only once.
	checkNumberOfRcvMsgOnGCM(t, restartedServiceParam, 1)
	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 0)
	time.Sleep(time.Millisecond * 150)

	// //clean up
	defer func() {
		err = restartedServiceParam.service.Stop()
		a.NoError(err)

		err = secondServiceParam.service.Stop()
		a.NoError(err)

		errRemove := os.RemoveAll(restartedServiceParam.dir)
		a.NoError(errRemove)
		errRemove = os.RemoveAll(secondServiceParam.dir)
		a.NoError(errRemove)
	}()
}

func Test_independent_Receiveing(t *testing.T) {
	//TODO MARIAN   removed this after stop is working
	testutil.SkipIfShort(t)
	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	firstServiceParam := startService(t, "8080", "dir1", 1, 10000, false)
	a.NotNil(firstServiceParam)

	secondServiceParam := startService(t, "8081", "dir2", 2, 10001, false)
	a.NotNil(secondServiceParam)

	//clean-up
	defer func() {
		err := firstServiceParam.service.Stop()
		a.NoError(err)

		// err = secondServiceParam.service.Stop()
		// a.NoError(err)

		err = os.RemoveAll(firstServiceParam.dir)
		a.NoError(err)

		err = os.RemoveAll(secondServiceParam.dir)
		a.NoError(err)
	}()

	//subscribe on first node
	subscribeToServiceNode(t, firstServiceParam.service)

	//connect a clinet and send a message
	client1, err1 := client.Open("ws://"+firstServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
	a.NoError(err1)

	err := client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	//only one message should be received but only on the first node.Every message should be delivered only once.
	checkNumberOfRcvMsgOnGCM(t, firstServiceParam, 1)
	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 0)

	// reset the counter
	firstServiceParam.received = 0
	// subscribeToServiceNode(t, secondServiceParam.service)

	//NOW connect to second node
	client1, err1 = client.Open("ws://"+secondServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8081", 1000, true)
	a.NoError(err1)

	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	//only one message should be received but only on the first node.Every message should be delivered only once.
	checkNumberOfRcvMsgOnGCM(t, firstServiceParam, 0)
	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 1)
}
