package cluster

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"

	"errors"
	"testing"
	"time"
)

func TestCluster_StartCheckStop(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10000, Remotes: []string{"127.0.0.1:10000"}}
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.MessageHandler = DummyMessageHandler{}

	err = node.Start()
	a.NoError(err, "No error should be raised when Starting the Cluster")

	err = node.Check()
	a.NoError(err, "Health-check score of a Cluster with a single node should be OK")

	err = node.Stop()
	a.NoError(err, "No error should be raised when Stopping the Cluster")
}

func TestCluster_BroadcastStringAndMessageAndCheck(t *testing.T) {
	defer testutil.EnableDebugForMethod()
	a := assert.New(t)

	confNode1 := Config{ID: 1, Host: "127.0.0.1", Port: 10001, Remotes: []string{"127.0.0.1:10001"}}
	node1, err := New(&confNode1)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node1.MessageHandler = DummyMessageHandler{}

	//start the cluster node 1
	defer node1.Stop()
	err = node1.Start()
	a.NoError(err, "No error should be raised when starting node 1 of the Cluster")

	confNode2 := Config{ID: 2, Host: "127.0.0.1", Port: 10002, Remotes: []string{"127.0.0.1:10001"}}
	node2, err := New(&confNode2)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node2.MessageHandler = DummyMessageHandler{}

	//start the cluster node 2
	defer node2.Stop()
	err = node2.Start()
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
	a.NoError(err, "No error should be raised when sending a protocol message to Cluster")

	err = node1.Check()
	a.NoError(err, "Health-check score of a Cluster with 2 nodes should be OK for node 1")

	err = node2.Check()
	a.NoError(err, "Health-check score of a Cluster with 2 nodes should be OK for node 2")
}

func TestCluster_NewShouldReturnErrorWhenPortIsInvalid(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: -10, Remotes: []string{"127.0.0.1:9999"}}
	_, err := New(&conf)
	if a.Error(err, "An error was expected when Creating the Cluster") {
		a.Equal(err, errors.New("Failed to start TCP listener. Err: listen tcp :-10: bind: invalid argument"),
			"Error should be precisely defined")
	}
}

func TestCluster_StartShouldReturnErrorWhenNoRemotes(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10003, Remotes: []string{}}
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.MessageHandler = DummyMessageHandler{}

	defer node.Stop()
	err = node.Start()
	if a.Error(err, "An error is expected when Starting the Cluster") {
		a.Equal(err, errors.New("No remote hosts were successfuly contacted when this node wanted to join the cluster"),
			"Error should be precisely defined")
	}
}

func TestCluster_StartShouldReturnErrorWhenInvalidRemotes(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10004, Remotes: []string{"127.0.0.1:1"}}
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.MessageHandler = DummyMessageHandler{}

	defer node.Stop()
	err = node.Start()
	if a.Error(err, "An error is expected when Starting the Cluster") {
		expected := multierror.Append(errors.New("Failed to join 127.0.0.1: dial tcp 127.0.0.1:1: getsockopt: connection refused"))
		a.Equal(err, expected, "Error should be precisely defined")
	}
}

func TestCluster_StartShouldReturnErrorWhenNoMessageHandler(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10005, Remotes: []string{"127.0.0.1:10005"}}
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	defer node.Stop()
	err = node.Start()
	if a.Error(err, "An error is expected when Starting the Cluster") {
		expected := errors.New("There should be a valid MessageHandler already set-up")
		a.Equal(err, expected, "Error should be precisely defined")
	}
}

func TestCluster_NotifyMsgShouldSimplyReturnWhenDecodingInvalidMessage(t *testing.T) {
	a := assert.New(t)

	conf := Config{ID: 1, Host: "localhost", Port: 10008, Remotes: []string{"127.0.0.1:10008"}}
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.MessageHandler = DummyMessageHandler{}

	defer node.Stop()
	err = node.Start()
	a.NoError(err, "No error should be raised when Starting the Cluster")

	node.NotifyMsg([]byte{})

	//TODO Cosmin check that HandleMessage is not invoked (i.e. invalid message is not dispatched)
}

type DummyMessageHandler struct {
}

func (_ DummyMessageHandler) HandleMessage(pmsg *protocol.Message) error {
	return nil
}
