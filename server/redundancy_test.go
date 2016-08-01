package server

import (
	"time"

	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"

	"testing"
)

func Test_Subscribe_on_random_node(t *testing.T) {
	defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	node1 := NewTestClusterNode(t, TestNodeConfig{
		HttpListen: ":8080",
		NodeID:     1,
		NodePort:   10000,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node1)
	defer node1.Cleanup()

	node2 := NewTestClusterNode(t, TestNodeConfig{
		HttpListen: ":8081",
		NodeID:     2,
		NodePort:   10001,
		Remotes:    []string{"127.0.0.1:10000"},
	})
	a.NotNil(node2)
	defer node2.Cleanup()

	node1.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)
	node2.GCM.SetupRoundTripper(20*time.Millisecond, 10, testutil.SuccessGCMResponse)

	//subscribe on first node
	node1.Subscribe(gcmTopic)
	node2.Subscribe(gcmTopic)

	//connect a clinet and send a message
	client1, err := node1.Client("user1", 1000, true)
	a.NoError(err)

	// only one message should be received but only on the first node.
	// Every message should be delivered only once.
	node1.GCM.CheckReceived(1, time.Second)
	node2.GCM.CheckReceived(0, time.Second)

	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
	a.NoError(err)
}

// func Test_Subscribe_working_After_Node_Restart(t *testing.T) {
// 	a := assert.New(t)

// 	firstServiceParam := startService(t, "8082", 1, 10000, "")
// 	a.NotNil(firstServiceParam)

// 	secondServiceParam := startService(t, "8083", 2, 10001, "")
// 	a.NotNil(secondServiceParam)
// 	//subscribe on first node
// 	subscribeToServiceNode(t, firstServiceParam.service)

// 	//connect a clinet and send a message
// 	client1, err1 := client.Open("ws://"+firstServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
// 	a.NoError(err1)

// 	err := client1.Send(gcmTopic, "body", "{jsonHeader:1}")
// 	a.NoError(err)

// 	//only one message should be received but only on the first node.Every message should be delivered only once.
// 	checkNumberOfRcvMsgOnGCM(t, firstServiceParam, 1)
// 	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 0)

// 	// stop a node
// 	err = firstServiceParam.service.Stop()
// 	a.NoError(err)
// 	time.Sleep(time.Millisecond * 150)

// 	// // restart the service
// 	restartedServiceParam := startService(t, "8082", 1, 10000, firstServiceParam.dir)
// 	a.NotNil(restartedServiceParam)

// 	//   send a message to the former subscription.
// 	client1, err1 = client.Open("ws://"+restartedServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
// 	a.NoError(err1)

// 	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
// 	a.NoError(err, "Subscription should work even after node restart")

// 	//only one message should be received but only on the first node.Every message should be delivered only once.
// 	checkNumberOfRcvMsgOnGCM(t, restartedServiceParam, 1)
// 	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 0)
// 	time.Sleep(time.Millisecond * 150)

// 	// //clean up
// 	err = restartedServiceParam.service.Stop()
// 	a.NoError(err)

// 	err = secondServiceParam.service.Stop()
// 	a.NoError(err)

// 	errRemove := os.RemoveAll(restartedServiceParam.dir)
// 	a.NoError(errRemove)
// 	errRemove = os.RemoveAll(secondServiceParam.dir)
// 	a.NoError(errRemove)
// }

// func Test_Independent_Receiving(t *testing.T) {
// 	a := assert.New(t)

// 	firstServiceParam := startService(t, "8084", 1, 10000, "")
// 	a.NotNil(firstServiceParam)

// 	secondServiceParam := startService(t, "8085", 2, 10001, "")
// 	a.NotNil(secondServiceParam)

// 	//subscribe on first node
// 	subscribeToServiceNode(t, firstServiceParam.service)

// 	//connect a clinet and send a message
// 	client1, err1 := client.Open("ws://"+firstServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8080", 1000, true)
// 	a.NoError(err1)

// 	err := client1.Send(gcmTopic, "body", "{jsonHeader:1}")
// 	a.NoError(err)

// 	//only one message should be received but only on the first node.Every message should be delivered only once.
// 	checkNumberOfRcvMsgOnGCM(t, firstServiceParam, 1)
// 	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 0)

// 	// reset the counter
// 	firstServiceParam.received = 0
// 	// subscribeToServiceNode(t, secondServiceParam.service)

// 	//NOW connect to second node
// 	client1, err1 = client.Open("ws://"+secondServiceParam.service.WebServer().GetAddr()+"/stream/user/", "http://localhost:8081", 1000, true)
// 	a.NoError(err1)

// 	err = client1.Send(gcmTopic, "body", "{jsonHeader:1}")
// 	a.NoError(err)

// 	//only one message should be received but only on the first node.Every message should be delivered only once.
// 	checkNumberOfRcvMsgOnGCM(t, firstServiceParam, 0)
// 	checkNumberOfRcvMsgOnGCM(t, secondServiceParam, 1)

// 	//clean-up
// 	err = firstServiceParam.service.Stop()
// 	a.NoError(err)

// 	err = secondServiceParam.service.Stop()
// 	a.NoError(err)

// 	err = os.RemoveAll(firstServiceParam.dir)
// 	a.NoError(err)

// 	err = os.RemoveAll(secondServiceParam.dir)
// 	a.NoError(err)
// }
