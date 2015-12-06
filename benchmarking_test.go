package main

import (
	assert "github.com/stretchr/testify/assert"
	"testing"

	"fmt"
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"log"
	"time"
)

type testgroup struct {
	t              *testing.T
	groupId        int
	addr           string
	done           chan bool
	messagesToSend int
}

func newTestgroup(t *testing.T, groupId int, addr string, messagesToSend int) *testgroup {
	return &testgroup{
		t:              t,
		groupId:        groupId,
		addr:           addr,
		done:           make(chan bool),
		messagesToSend: messagesToSend,
	}
}

func (test *testgroup) Start() {
	var err error
	var client1, client2 *client.Client

	client1, err = client.Open("ws://"+test.addr, "http://localhost/", 10, true)
	assert.NoError(test.t, err)
	defer client1.Close()
	client2, err = client.Open("ws://"+test.addr, "http://localhost/", 10, true)
	assert.NoError(test.t, err)
	defer client1.Close()

	time.Sleep(time.Millisecond * 10)

	topic := fmt.Sprintf("/%v-foo", test.groupId)
	client1.Subscribe(topic)

	time.Sleep(time.Millisecond * 10)

	for i := 0; i < test.messagesToSend; i++ {
		body := fmt.Sprintf("Hallo-%v", i)
		client2.Send(topic, body)

		select {
		case msg := <-client1.Messages():
			assert.Equal(test.t, body, msg.BodyAsString())
			assert.Equal(test.t, topic, string(msg.Path))
		case msg := <-client1.Errors():
			test.t.Logf("received error: %v", msg)
			test.t.FailNow()
			//case msg := <-client1.StatusMessages():
			//	test.t.Logf("received status message: %v", msg)
		case <-time.After(time.Millisecond * 100):
			test.t.Log("no message received")
			test.t.FailNow()
		}
	}
	test.done <- true
}

func TestThroughput(t *testing.T) {
	guble.LogLevel = guble.LEVEL_ERR
	log.Print("start the server")
	mux := server.NewPubSubRouter().Go()
	wshandlerFactory := func(wsConn server.WSConn) server.Startable {
		return server.NewWSHandler(mux, mux, wsConn)
	}
	ws := server.StartWSServer("localhost:0", wshandlerFactory)
	defer func() {
		mux.Stop()
		ws.Stop()
	}()
	time.Sleep(time.Millisecond * 10)

	testgroupCount := 10
	messagesPerGroup := 1000
	log.Printf("init the %v testgroups", testgroupCount)
	testgroups := make([]*testgroup, testgroupCount, testgroupCount)
	for i, _ := range testgroups {
		testgroups[i] = newTestgroup(t, i, ws.GetAddr(), messagesPerGroup)
	}

	// start test
	log.Print("start the testgroups")
	start := time.Now()
	for i, _ := range testgroups {
		go testgroups[i].Start()
	}

	log.Print("wait for finishing")
	timeout := time.After(time.Second * 60)
	for _, test := range testgroups {
		select {
		case <-test.done:
		case <-timeout:
			t.Log("timeout. testgroups not ready before timeout")
			t.Fail()
			return
		}
	}

	end := time.Now()
	totalMessages := testgroupCount * messagesPerGroup
	throughput := float64(totalMessages) / end.Sub(start).Seconds()
	log.Printf("finished! Throughput: %v/sec (%v message in %v)", int(throughput), totalMessages, end.Sub(start))
}
