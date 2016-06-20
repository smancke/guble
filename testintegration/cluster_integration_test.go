package testintegration

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gubled"
	"github.com/smancke/guble/server"
	"github.com/stretchr/testify/assert"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"testing"
	"time"
	"github.com/smancke/guble/testutil"
)

func createService(storagePath, nodeID, nodePort, listenPort string, remotes string) *server.Service {
	os.Args = []string{os.Args[0],
		"--log", "debug",
		"--listen", listenPort,
		"--storage-path", storagePath,
		"--node-id", nodeID,
		"--health", "",
		"--node-port", nodePort,
		remotes,
	}

	kingpin.Parse()
	service := gubled.StartService()
	return service
}

func Test_Cluster(t *testing.T) {
	a := assert.New(t)
	defer testutil.EnableDebugForMethod()()


	service1 := createService("/tmp/s1", "1", "10000", "127.0.0.1:8080", "tcp://127.0.0.1:10000")
	a.NotNil(service1)

	service2 := createService("/tmp/s2", "2", "10001", "127.0.0.1:8081", "tcp://127.0.0.1:10000")
	a.NotNil(service2)

	client1, err := client.Open("ws://127.0.0.1:8081/stream/user/user1", "http://localhost", 1, false)
	assert.NoError(t, err)

	client2, err := client.Open("ws://127.0.0.1:8080/stream/user/user2", "http://localhost", 1, false)
	assert.NoError(t, err)

	err = client1.Subscribe("/foo")
	a.Nil(err)

	err = client2.Subscribe("/barbazzMarian35")
	a.Nil(err)

	err = client1.Send("/barbazzMarian35", "", "{}")
	a.Nil(err)

	time.Sleep(time.Millisecond * 10)
	//
	err = service1.Stop()
	//err = service2.Stop()
	time.Sleep(time.Second * 2)
	a.Nil(err)
}
