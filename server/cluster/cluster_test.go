package cluster

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCluster_StartStop(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10000, Remotes: []string{"127.0.0.1:10000"}}
	node := New(&conf)
	node.MessageHandler = DummyMessageHandler{}

	//start the cluster
	err := node.Start()
	a.Nil(err, "No error should be raised when Starting the Cluster")

	//stop the cluster
	err = node.Stop()
	a.Nil(err, "No error should be raised when Stopping the Cluster")
}

func TestCluster_BroadcastMessage(t *testing.T) {
	defer testutil.EnableDebugForMethod()
	a := assert.New(t)

	confNode1 := Config{ID: 1, Host: "localhost", Port: 10001, Remotes: []string{"127.0.0.1:10001"}}
	node1 := New(&confNode1)
	node1.MessageHandler = DummyMessageHandler{}

	//start the cluster node 1
	err := node1.Start()
	defer node1.Stop()
	a.Nil(err, "No error should be raised when Starting the Cluster (node 1)")

	confNode2 := Config{ID: 2, Host: "localhost", Port: 10002, Remotes: []string{"127.0.0.1:10002"}}
	node2 := New(&confNode2)
	node2.MessageHandler = DummyMessageHandler{}

	//start the cluster node 2
	err = node2.Start()
	defer node2.Stop()
	a.Nil(err, "No error should be raised when Starting the Cluster (node 2)")

	// Send a String Message
	str := "TEST"
	err = node1.BroadcastString(&str)
	a.Nil(err, "No error should be raised when sending a string to Cluster")

	// and a protocol message
	pmsg := protocol.Message{
		ID:            1,
		Path:          "/stuff",
		UserID:        "id",
		ApplicationID: "appId",
		Time:          time.Now().Unix(),
		HeaderJSON:    "{}",
		Body:          []byte("test"),
		NodeID:        1}
	err = node1.BroadcastMessage(&pmsg)
	a.Nil(err, "No error should be raised when sending a procotocol message to Cluster")
}

type DummyMessageHandler struct {
}

func (dmh DummyMessageHandler) HandleMessage(pmsg *protocol.Message) error {
	return nil
}
