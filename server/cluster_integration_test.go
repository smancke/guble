package server

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/service"
	"github.com/smancke/guble/testutil"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/alecthomas/kingpin.v2"

	"io/ioutil"
	"os"
	"testing"
	"time"
)

func createService(storagePath, nodeID, nodePort, httpListen string, remotes string) *service.Service {
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
	service := StartService()
	return service
}

func Test_Cluster_Integration(t *testing.T) {
	testutil.SkipIfShort(t)
	defer testutil.ResetDefaultRegistryHealthCheck()
	//defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	dir1, err1 := ioutil.TempDir("", "guble_cluster_integration_test")
	a.NoError(err1)
	defer os.RemoveAll(dir1)

	dir2, err2 := ioutil.TempDir("", "guble_cluster_integration_test")
	a.NoError(err2)
	defer os.RemoveAll(dir2)

	service1 := createService("/tmp/s3", "1", "10000", ":8080", "127.0.0.1:10000")
	a.NotNil(service1)

	service2 := createService("/tmp/s4", "2", "10001", ":8081", "127.0.0.1:10000")
	a.NotNil(service2)

	defer func() {
		errStop1 := service1.Stop()
		errStop2 := service2.Stop()
		a.NoError(errStop1)
		a.NoError(errStop2)
	}()

	client1, err1 := client.Open("ws://127.0.0.1:8080/stream/user/user1", "http://localhost", 10, false)
	a.NoError(err1)

	client2, err2 := client.Open("ws://localhost:8080/stream/user/user2", "http://localhost", 10, false)
	a.NoError(err2)

	err2 = client2.Subscribe("/testTopic/m")
	a.NoError(err2)

	client3, err3 := client.Open("ws://127.0.0.1:8080/stream/user/user3", "http://localhost", 10, false)
	a.NoError(err3)

	numSent := 3
	for i := 0; i < numSent; i++ {
		err := client1.Send("/testTopic/m", "body", "{jsonHeader:1}")
		a.NoError(err)

		err = client3.Send("/testTopic/m", "body", "{jsonHeader:4}")
		a.NoError(err)

		//TODO Cosmin this sleep should be eliminated when messages receive correct message-IDs
		//time.Sleep(time.Millisecond * 20)
	}
	breakTimer := time.After(3 * time.Second)
	numReceived := 0
	idReceived := make(map[uint64]bool)

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

			a.Equal(protocol.Path("/testTopic/m"), incomingMessage.Path)
			a.Equal("body", incomingMessage.BodyAsString())
			a.True(incomingMessage.ID > 0)
			idReceived[incomingMessage.ID] = true

			if 2*numReceived == numSent {
				break WAIT
			}

		case <-breakTimer:
			break WAIT
		}
	}

}
