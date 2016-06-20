package cluster

import (
	"errors"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCluster_StartStop(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10000, Remotes: []string{"127.0.0.1:10000"}}
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.MessageHandler = DummyMessageHandler{}

	//start the cluster
	err = node.Start()
	a.NoError(err, "No error should be raised when Starting the Cluster")

	//stop the cluster
	err = node.Stop()
	a.NoError(err, "No error should be raised when Stopping the Cluster")
}

func TestCluster_BroadcastMessage(t *testing.T) {
	defer testutil.EnableDebugForMethod()
	a := assert.New(t)

	confNode1 := Config{ID: 1, Host: "localhost", Port: 10001, Remotes: []string{"127.0.0.1:10001"}}
	node1, err := New(&confNode1)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node1.MessageHandler = DummyMessageHandler{}

	//start the cluster node 1
	err = node1.Start()
	defer node1.Stop()
	a.NoError(err, "No error should be raised when starting node 1 of the Cluster")

	confNode2 := Config{ID: 2, Host: "localhost", Port: 10002, Remotes: []string{"127.0.0.1:10001"}}
	node2, err := New(&confNode2)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node2.MessageHandler = DummyMessageHandler{}

	//start the cluster node 2
	err = node2.Start()
	defer node2.Stop()
	a.NoError(err, "No error should be raised when starting node 2 of the Cluster")

	// Send a String Message
	str := "TEST"
	err = node1.BroadcastString(&str)
	a.NoError(err, "No error should be raised when sending a string to Cluster")

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
	a.NoError(err, "No error should be raised when sending a procotocol message to Cluster")
}

func TestCluster_NewShouldReturnErrorWhenPortIsInvalid(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: -1, Remotes: []string{"127.0.0.1:10000"}}
	_, err := New(&conf)
	if a.Error(err, "An error was expected when Creating the Cluster") {
		a.Equal(err, errors.New("Failed to start TCP listener. Err: listen tcp :-1: bind: invalid argument"),
			"Error should be precisely defined")
	}
}

func TestCluster_StartShouldReturnErrorWhenNoRemotes(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10000, Remotes: []string{}}
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.MessageHandler = DummyMessageHandler{}

	err = node.Start()
	if a.Error(err, "An error is expected when Starting the Cluster") {
		a.Equal(err, errors.New("No remote hosts were successfuly contacted when this node wanted to join the cluster"),
			"Error should be precisely defined")
	}

	err = node.Stop()
	a.NoError(err, "No error should be raised when Stopping the Cluster")
}

type DummyMessageHandler struct {
}

func (dmh DummyMessageHandler) HandleMessage(pmsg *protocol.Message) error {
	return nil
}
