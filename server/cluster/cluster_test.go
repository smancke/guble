package cluster

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
