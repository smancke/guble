package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"testing"
	"time"
)

func Test_Cluster_Subscribe_To_Random_Node(t *testing.T) {
	testutil.SkipIfShort(t)
	a := assert.New(t)

	node1 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: ":8080",
		NodeID:     1,
		NodePort:   10000,
		Remotes:    "127.0.0.1:10000",
	})
	a.NotNil(node1)
	defer node1.cleanup(true)

	node2 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: ":8081",
		NodeID:     2,
		NodePort:   10001,
		Remotes:    "127.0.0.1:10000",
	})
	a.NotNil(node2)
	defer node2.cleanup(true)

	client1, err := node1.client("user1", 10, true)
	a.NoError(err)

	err = client1.Subscribe("/foo/bar")
	a.NoError(err, "Subscribe to first node should work")

	client1.Close()

	time.Sleep(50 * time.Millisecond)

	client1, err = node2.client("user1", 10, true)
	a.NoError(err, "Connection to second node should return no error")

	err = client1.Subscribe("/foo/bar")
	a.NoError(err, "Subscribe to second node should work")
	client1.Close()
}

func Test_Cluster_Integration(t *testing.T) {
	testutil.SkipIfShort(t)
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(t)

	node1 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: ":8080",
		NodeID:     1,
		NodePort:   10000,
		Remotes:    "127.0.0.1:10000",
	})
	a.NotNil(node1)
	defer node1.cleanup(true)

	node2 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: ":8081",
		NodeID:     2,
		NodePort:   10001,
		Remotes:    "127.0.0.1:10000",
	})
	a.NotNil(node2)
	defer node2.cleanup(true)

	client1, err := node1.client("user1", 10, false)
	a.NoError(err)

	client2, err := node2.client("user2", 10, false)
	a.NoError(err)

	err = client2.Subscribe("/testTopic/m")
	a.NoError(err)

	client3, err := node1.client("user3", 10, false)
	a.NoError(err)

	numSent := 3
	for i := 0; i < numSent; i++ {
		err := client1.Send("/testTopic/m", "body", "{jsonHeader:1}")
		a.NoError(err)

		err = client3.Send("/testTopic/m", "body", "{jsonHeader:4}")
		a.NoError(err)
	}

	breakTimer := time.After(3 * time.Second)
	numReceived := 0
	idReceived := make(map[uint64]bool)

	// see if the correct number of messages arrived at the other client, before timeout is reached
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

var syncTopic = "/sync"

// Test synchronizing messages when a new node is
func TestSynchronizerIntegration(t *testing.T) {
	testutil.SkipIfShort(t)
	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	node1 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: ":8080",
		NodeID:     1,
		NodePort:   10000,
		Remotes:    "127.0.0.1:10000",
	})
	a.NotNil(node1)
	defer node1.cleanup(true)

	time.Sleep(2 * time.Second)

	client1, err := node1.client("client1", 10, true)
	a.NoError(err)

	client1.Send(syncTopic, "nobody", "")
	client1.Send(syncTopic, "nobody", "")
	client1.Send(syncTopic, "nobody", "")

	time.Sleep(2 * time.Second)

	node2 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: ":8081",
		NodeID:     2,
		NodePort:   10001,
		Remotes:    "127.0.0.1:10000",
	})
	a.NotNil(node2)
	defer node2.cleanup(true)
}
