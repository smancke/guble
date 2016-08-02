package server

import (
	"time"

	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"

	"testing"
)

func Test_Subscribe_on_random_node(t *testing.T) {
	a := assert.New(t)

	node1 := NewTestClusterNode(t, TestClusterNodeConfig{
		HttpListen: "0.0.0.0:8080",
		NodeID:     1,
		NodePort:   10000,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node1)
	defer node1.Cleanup(true)

	node2 := NewTestClusterNode(t, TestClusterNodeConfig{
		HttpListen: "0.0.0.0:8081",
		NodeID:     2,
		NodePort:   10001,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node2)
	defer node2.Cleanup(true)

	node1.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)
	node2.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)

	//subscribe on first node
	node1.Subscribe(gcmTopic)
	node2.Subscribe(gcmTopic)

	//connect a clinet and send a message
	client1, err := node1.Client("user1", 1000, true)
	a.NoError(err)

	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	// only one message should be received but only on the first node.
	// Every message should be delivered only once.
	node1.GCM.CheckReceived(1)
	node2.GCM.CheckReceived(0)

}

func Test_Subscribe_working_After_Node_Restart(t *testing.T) {
	a := assert.New(t)

	node1 := NewTestClusterNode(t, TestClusterNodeConfig{
		HttpListen: "0.0.0.0:8082",
		NodeID:     1,
		NodePort:   10000,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node1)

	node2 := NewTestClusterNode(t, TestClusterNodeConfig{
		HttpListen: "0.0.0.0:8083",
		NodeID:     2,
		NodePort:   10001,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node2)
	defer node2.Cleanup(true)

	node1.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)
	node2.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)

	// subscribe on first node
	node1.Subscribe(gcmTopic)

	// connect a clinet and send a message
	client1, err := node1.Client("user1", 1000, true)
	a.NoError(err)
	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	// one message should be received but only on the first node.
	// Every message should be delivered only once.
	node1.GCM.CheckReceived(1)
	node2.GCM.CheckReceived(0)

	// stop a node, cleanup without removing directories
	node1.Cleanup(false)
	time.Sleep(time.Millisecond * 150)

	// restart the service
	restartedNode1 := NewTestClusterNode(t, TestClusterNodeConfig{
		HttpListen:  ":8082",
		StoragePath: node1.StoragePath,
		NodeID:      1,
		NodePort:    10000,
		Remotes:     []string{"127.0.0.1:10000"},
	})
	a.NotNil(restartedNode1)
	defer restartedNode1.Cleanup(true)

	restartedNode1.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)

	// send a message to the former subscription.
	client1, err = restartedNode1.Client("user1", 1000, true)
	a.NoError(err)

	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err, "Subscription should work even after node restart")
	time.Sleep(time.Second)

	// only one message should be received but only on the first node.
	// Every message should be delivered only once.
	restartedNode1.GCM.CheckReceived(1)
	node2.GCM.CheckReceived(0)
}

func Test_Independent_Receiving(t *testing.T) {
	a := assert.New(t)

	node1 := NewTestClusterNode(t, TestClusterNodeConfig{
		HttpListen: "0.0.0.0:8084",
		NodeID:     1,
		NodePort:   10000,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node1)
	defer node1.Cleanup(true)

	node2 := NewTestClusterNode(t, TestClusterNodeConfig{
		HttpListen: "0.0.0.0:8085",
		NodeID:     2,
		NodePort:   10001,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node2)
	defer node2.Cleanup(true)

	node1.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)
	node2.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)

	// subscribe on first node
	node1.Subscribe(gcmTopic)

	// connect a clinet and send a message
	client1, err := node1.Client("user1", 1000, true)
	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	// only one message should be received but only on the first node.
	// Every message should be delivered only once.
	node1.GCM.CheckReceived(1)
	node2.GCM.CheckReceived(0)

	// reset the counter
	node1.GCM.Reset()

	// NOW connect to second node
	client2, err := node2.Client("user2", 1000, true)
	a.NoError(err)
	err = client2.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)

	// only one message should be received but only on the first node.
	// Every message should be delivered only once.
	node1.GCM.CheckReceived(0)
	node2.GCM.CheckReceived(1)
}
