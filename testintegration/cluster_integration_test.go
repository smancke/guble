package testintegration

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gubled"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/alecthomas/kingpin.v2"

	"io/ioutil"
	"os"
	"testing"
	"time"
)

func createService(storagePath, nodeID, nodePort, httpListen string, remotes string) *server.Service {
	os.Args = []string{os.Args[0],
		"--log", "debug",
		"--http", httpListen,
		"--storage-path", storagePath,
		"--node-id", nodeID,
		"--health-endpoint", "",
		"--node-port", nodePort,
		remotes,
	}

	kingpin.Parse()
	service := gubled.StartService()
	return service
}

func Test_Cluster(t *testing.T) {
	a := assert.New(t)
	//defer testutil.EnableDebugForMethod()()

	dir1, err1 := ioutil.TempDir("", "guble_cluster_integration_test")
	a.NoError(err1)
	defer os.RemoveAll(dir1)

	dir2, err2 := ioutil.TempDir("", "guble_cluster_integration_test")
	a.NoError(err2)
	defer os.RemoveAll(dir2)

	service1 := createService(dir1, "1", "10000", ":8080", "127.0.0.1:10000")
	a.NotNil(service1)

	service2 := createService(dir2, "2", "10001", ":8081", "127.0.0.1:10000")
	a.NotNil(service2)

	defer func() {
		errStop1 := service1.Stop()
		errStop2 := service2.Stop()
		a.NoError(errStop1)
		a.NoError(errStop2)
	}()

	client1, err1 := client.Open("ws://127.0.0.1:8081/stream/user/user1", "http://localhost", 10, false)
	a.NoError(err1)

	client2, err2 := client.Open("ws://127.0.0.1:8080/stream/user/user2", "http://localhost", 10, false)
	a.NoError(err2)

	err2 = client2.Subscribe("/testTopic")
	a.NoError(err2)

	numSent := 5
	for i := 0; i < numSent; i++ {
		err := client1.Send("/testTopic", "body", "{jsonHeader:1}")
		a.NoError(err)

		//TODO Cosmin this sleep should be eliminated when messages receive correct message-IDs
		time.Sleep(time.Millisecond * 20)
	}

	breakTimer := time.After(time.Second)
	numReceived := 0

	//see if the correct number of messages arrived at the other client, before timeout is reached
WAIT:
	for {
		select {
		case incomingMessage := <-client2.Messages():
			numReceived++
			logger.WithFields(log.Fields{
				"nodeID":            incomingMessage.NodeID,
				"path":              incomingMessage.Path,
				"incomingMsgUserId": incomingMessage.UserID,
				"headerJson":        incomingMessage.HeaderJSON,
				"body":              incomingMessage.BodyAsString(),
				"numReceived":       numReceived,
			}).Info("Client2 received a message")

			a.Equal(2, incomingMessage.NodeID)
			a.Equal(protocol.Path("/testTopic"), incomingMessage.Path)
			a.Equal("user1", incomingMessage.UserID)
			a.Equal("{jsonHeader:1}", incomingMessage.HeaderJSON)
			a.Equal("body", incomingMessage.BodyAsString())

			if numReceived == numSent {
				break WAIT
			}

		case <-breakTimer:
			break WAIT
		}
	}

	a.True(numReceived == numSent)
}
