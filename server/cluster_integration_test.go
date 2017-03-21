package server

import (
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
)

func Test_Cluster_Subscribe_To_Random_Node(t *testing.T) {
	testutil.SkipIfShort(t)
	a := assert.New(t)

	node1 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: "localhost:8090",
		NodeID:     1,
		NodePort:   11000,
		Remotes:    "localhost:11000",
	})
	a.NotNil(node1)
	defer node1.cleanup(true)

	node2 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: "localhost:8091",
		NodeID:     2,
		NodePort:   11001,
		Remotes:    "localhost:11000",
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
		HttpListen: "localhost:8092",
		NodeID:     1,
		NodePort:   11002,
		Remotes:    "localhost:11002",
	})
	a.NotNil(node1)
	defer node1.cleanup(true)

	node2 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: "localhost:8093",
		NodeID:     2,
		NodePort:   11003,
		Remotes:    "localhost:11002",
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
				"body":              string(incomingMessage.Body),
				"numReceived":       numReceived,
			}).Info("Client2 received a message")

			a.Equal(protocol.Path("/testTopic/m"), incomingMessage.Path)
			a.Equal("body", string(incomingMessage.Body))
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

var syncTopic = "/syncTopic"

// Test synchronizing messages when a new node is
func TestSynchronizerIntegration(t *testing.T) {
	testutil.SkipIfShort(t)
	//TODO REACTIVATE THIS AND see if it is working for future
	testutil.SkipIfDisabled(t)
	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	node1 := newTestClusterNode(t, testClusterNodeConfig{
		HttpListen: "localhost:8094",
		NodeID:     1,
		NodePort:   11004,
		Remotes:    "localhost:11004",
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
		HttpListen: "localhost:8095",
		NodeID:     2,
		NodePort:   11005,
		Remotes:    "localhost:11004",
	})
	a.NotNil(node2)
	defer node2.cleanup(true)

	client2, err := node2.client("client2", 10, true)
	a.NoError(err)

	cmd := &protocol.Cmd{
		Name: protocol.CmdReceive,
		Arg:  syncTopic + " -3",
	}
	doneC := make(chan struct{})
	go func() {
		for {
			select {
			case m := <-client2.Messages():
				log.WithField("m", m).Error("Message received from first cluster")
			case e := <-client2.Errors():
				log.WithField("clientError", e).Error("Client error")
			case status := <-client2.StatusMessages():
				log.WithField("status", status).Error("Client status messasge")
			case <-doneC:
				return
			}
		}
	}()
	log.Error(string(cmd.Bytes()))
	client2.WriteRawMessage(cmd.Bytes())
	time.Sleep(10 * time.Second)
	close(doneC)
}
