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
	cl := New(&conf)

	//start the cluster
	err := cl.Start()
	a.Nil(err, "No error should be raised when Starting the Cluster")

	//stop the cluster
	err = cl.Stop()
	a.Nil(err, "No error should be raised when Stopping the Cluster")
}

func TestCluster_BroadcastMessage(t *testing.T) {
	defer testutil.EnableDebugForMethod()
	a := assert.New(t)

	confNode1 := Config{ID: 1, Host: "localhost", Port: 10000, Remotes: []string{"127.0.0.1:10000"}}
	node1 := New(&confNode1)

	//start the cluster
	err := node1.Start()
	a.Nil(err, "No error should be raised when Starting the Cluster")

	confNode2 := Config{ID: 1, Host: "localhost", Port: 10001, Remotes: []string{"127.0.0.1:10000"}}
	node2 := New(&confNode2)
	//start the cluster
	err = node2.Start()
	a.Nil(err, "No error should be raised when Starting the Cluster")

	// Send a String Message
	str := "TEST"
	err = node1.BroadcastString(&str)
	a.Nil(err, "No error should be raised when sending to Cluster")

	// end a protocol message
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
	a.Nil(err, "No error should be raised when sending procotocol message to Cluster")
}
