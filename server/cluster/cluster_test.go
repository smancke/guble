package cluster

import (
	"github.com/cosminrentea/gobbler/server/store/filestore"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/store"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"

	"errors"
	"io/ioutil"
	"net"
	"testing"
	"time"
)

const basePort = 10000

var (
	index = 1
)

func testConfig() (config Config) {
	remoteAddr := net.TCPAddr{IP: []byte{127, 0, 0, 1}, Port: basePort + index}
	var remotes []*net.TCPAddr
	remotes = append(remotes, &remoteAddr)
	config = Config{ID: uint8(index), Host: "127.0.0.1", Port: basePort + index, Remotes: remotes}
	index++
	return
}

func testConfigAnother() (config Config) {
	remoteAddr := net.TCPAddr{IP: []byte{127, 0, 0, 1}, Port: basePort + index - 1}
	var remotes []*net.TCPAddr
	remotes = append(remotes, &remoteAddr)
	config = Config{ID: uint8(index), Host: "127.0.0.1", Port: basePort + index, Remotes: remotes}
	index++
	return
}

func TestCluster_StartCheckStop(t *testing.T) {
	a := assert.New(t)

	conf := testConfig()
	node, err := New(&conf)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.Router = newDummyRouter(t)

	err = node.Start()
	defer func() {
		err := node.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
	a.NoError(err, "No error should be raised when Starting the Cluster")

	err = node.Check()
	a.NoError(err, "Health-check score of a Cluster with a single node should be OK")
}

func TestCluster_BroadcastStringAndMessageAndCheck(t *testing.T) {
	a := assert.New(t)

	config1 := testConfig()
	node1, err := New(&config1)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node1.Router = newDummyRouter(t)

	//start the cluster node 1
	err = node1.Start()
	defer func() {
		err := node1.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
	a.NoError(err, "No error should be raised when starting node 1 of the Cluster")

	config2 := testConfigAnother()
	node2, err := New(&config2)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node2.Router = newDummyRouter(t)

	//start the cluster node 2
	err = node2.Start()
	defer func() {
		err := node2.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
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

	remoteAddr := net.TCPAddr{IP: []byte{127, 0, 0, 1}, Port: basePort + index - 1}
	var remotes []*net.TCPAddr
	remotes = append(remotes, &remoteAddr)
	index++

	config := Config{ID: 1, Host: "localhost", Port: -1, Remotes: remotes}
	_, err := New(&config)
	a.Error(err, "An error was expected when Creating the Cluster")
}

func TestCluster_StartShouldReturnErrorWhenNoRemotes(t *testing.T) {
	a := assert.New(t)

	var remotes []*net.TCPAddr
	index++

	config := Config{ID: 1, Host: "localhost", Port: basePort + index - 1, Remotes: remotes}
	node, err := New(&config)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.Router = newDummyRouter(t)

	err = node.Start()
	defer func() {
		err := node.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
	if a.Error(err, "An error is expected when Starting the Cluster") {
		a.Equal(err, errors.New("No remote hosts were successfully contacted when this node wanted to join the cluster"),
			"Error should be precisely defined")
	}
}

func TestCluster_StartShouldReturnErrorWhenInvalidRemotes(t *testing.T) {
	a := assert.New(t)

	remoteAddr := net.TCPAddr{IP: []byte{127, 0, 0, 1}, Port: 0}
	var remotes []*net.TCPAddr
	remotes = append(remotes, &remoteAddr)
	index++

	config := Config{ID: 1, Host: "localhost", Port: basePort + index - 1, Remotes: remotes}
	node, err := New(&config)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.Router = newDummyRouter(t)

	err = node.Start()
	defer func() {
		err := node.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
	if a.Error(err, "An error is expected when Starting the Cluster") {
		expected := multierror.Append(errors.New("Failed to join 127.0.0.1: dial tcp 127.0.0.1:0: getsockopt: connection refused"))
		a.Equal(err, expected, "Error should be precisely defined")
	}
}

func TestCluster_StartShouldReturnErrorWhenNoMessageHandler(t *testing.T) {
	a := assert.New(t)

	config := testConfig()
	node, err := New(&config)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	err = node.Start()
	defer func() {
		err := node.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
	if a.Error(err, "An error is expected when Starting the Cluster") {
		expected := errors.New("There should be a valid Router already set-up")
		a.Equal(expected, err, "Error should be precisely defined")
	}
}

func TestCluster_NotifyMsgShouldSimplyReturnWhenDecodingInvalidMessage(t *testing.T) {
	a := assert.New(t)

	config := testConfig()
	node, err := New(&config)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.Router = newDummyRouter(t)

	err = node.Start()
	defer func() {
		err := node.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
	a.NoError(err, "No error should be raised when Starting the Cluster")

	node.NotifyMsg([]byte{})

	//TODO Cosmin check that HandleMessage is not invoked (i.e. invalid message is not dispatched)
}

func TestCluster_broadcastClusterMessage(t *testing.T) {
	a := assert.New(t)

	config := testConfig()
	node, err := New(&config)
	a.NoError(err, "No error should be raised when Creating the Cluster")

	node.Router = newDummyRouter(t)

	err = node.Start()
	defer func() {
		err := node.Stop()
		a.NoError(err, "No error should be raised when Stopping the Cluster")
	}()
	a.NoError(err, "No error should be raised when Starting the Cluster")

	err = node.broadcastClusterMessage(nil)
	if a.Error(err, "An error is expected from broadcastClusterMessage") {
		expected := errors.New("Could not broadcast a nil cluster-message")
		a.Equal(err, expected, "Error should be precisely defined")
	}
}

type dummyRouter struct {
	store store.MessageStore
}

func newDummyRouter(t *testing.T) *dummyRouter {
	dir, err := ioutil.TempDir("", "guble_cluster_test")
	assert.NoError(t, err)
	return &dummyRouter{store: filestore.New(dir)}
}

func (_ *dummyRouter) HandleMessage(pmsg *protocol.Message) error {
	return nil
}

func (d *dummyRouter) MessageStore() (store.MessageStore, error) {
	return d.store, nil
}
