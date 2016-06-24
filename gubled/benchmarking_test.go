package gubled

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/gubled/config"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"
)

type testgroup struct {
	t                *testing.T
	groupID          int
	addr             string
	done             chan bool
	messagesToSend   int
	client1, client2 client.Client
	topic            string
}

func newTestgroup(t *testing.T, groupID int, addr string, messagesToSend int) *testgroup {
	return &testgroup{
		t:              t,
		groupID:        groupID,
		addr:           addr,
		done:           make(chan bool),
		messagesToSend: messagesToSend,
	}
}

func TestThroughput(t *testing.T) {
	//testutil.EnableDebugForMethod()()
	defer testutil.ResetDefaultRegistryHealthCheck()

	dir, _ := ioutil.TempDir("", "guble_benchmarking_test")
	defer os.RemoveAll(dir)

	*config.HttpListen = "localhost:0"
	*config.KVS = "memory"
	*config.MS = "file"
	*config.StoragePath = dir

	service := StartService()
	defer service.Stop()

	time.Sleep(time.Millisecond * 10)

	testgroupCount := 4
	messagesPerGroup := 100
	log.Printf("init the %v testgroups", testgroupCount)
	testgroups := make([]*testgroup, testgroupCount, testgroupCount)
	for i := range testgroups {
		testgroups[i] = newTestgroup(t, i, service.WebServer().GetAddr(), messagesPerGroup)
	}

	// init test
	log.Print("init the testgroups")
	for i := range testgroups {
		testgroups[i].Init()
	}

	defer func() {
		// cleanup tests
		log.Print("cleanup the testgroups")
		for i := range testgroups {
			testgroups[i].Clean()
		}
	}()

	// start test after filesystem sync
	assert.NoError(t, testutil.SyncStorage())

	log.Print("start the testgroups")
	start := time.Now()
	for i := range testgroups {
		go testgroups[i].Start()
	}

	log.Print("wait for finishing")
	timeout := time.After(time.Second * 60)
	for i, test := range testgroups {
		//fmt.Printf("waiting for test %v\n", i)
		select {
		case successFlag := <-test.done:
			if !successFlag {
				t.Logf("testgroup %v returned with error", i)
				t.FailNow()
				return
			}
		case <-timeout:
			t.Log("timeout. testgroups not ready before timeout")
			t.Fail()
			return
		}
	}

	assert.NoError(t, testutil.SyncStorage())
	end := time.Now()
	totalMessages := testgroupCount * messagesPerGroup
	throughput := float64(totalMessages) / end.Sub(start).Seconds()
	log.Printf("Finished! Throughput: %v/sec (%v message in %v)", int(throughput), totalMessages, end.Sub(start))
}

func (tg *testgroup) Init() {
	tg.topic = fmt.Sprintf("/%v-foo", tg.groupID)
	var err error
	location := "ws://" + tg.addr + "/stream/user/xy"
	//location := "ws://127.0.0.1:8080/stream/"
	tg.client1, err = client.Open(location, "http://localhost/", 10, false)
	if err != nil {
		panic(err)
	}
	tg.client2, err = client.Open(location, "http://localhost/", 10, false)
	if err != nil {
		panic(err)
	}

	tg.expectStatusMessage(protocol.SUCCESS_CONNECTED, "You are connected to the server.")

	tg.client1.Subscribe(tg.topic)
	time.Sleep(time.Millisecond * 1)
	//test.expectStatusMessage(protocol.SUCCESS_SUBSCRIBED_TO, test.topic)
}

func (tg *testgroup) expectStatusMessage(name string, arg string) {
	select {
	case notify := <-tg.client1.StatusMessages():
		assert.Equal(tg.t, name, notify.Name)
		assert.Equal(tg.t, arg, notify.Arg)
	case <-time.After(time.Second * 1):
		tg.t.Logf("[%v] no notification of type %s until timeout", tg.groupID, name)
		tg.done <- false
		tg.t.Fail()
		return
	}
}

func (tg *testgroup) Start() {
	go func() {
		for i := 0; i < tg.messagesToSend; i++ {
			// message body has constant length
			body := fmt.Sprintf("%16d", i)
			tg.client2.Send(tg.topic, body, "")
		}
	}()

	for i := 0; i < tg.messagesToSend; i++ {
		body := fmt.Sprintf("%16d", i)

		select {
		case msg := <-tg.client1.Messages():
			assert.Equal(tg.t, body, msg.BodyAsString())
			assert.Equal(tg.t, tg.topic, string(msg.Path))
		case msg := <-tg.client1.Errors():
			tg.t.Logf("[%v] received error: %v", tg.groupID, msg)
			tg.done <- false
			tg.t.Fail()
			return
		case <-time.After(time.Second * 5):
			tg.t.Logf("[%v] no message received until timeout, expected message %v", tg.groupID, i)
			tg.done <- false
			tg.t.Fail()
			return
		}
	}
	tg.done <- true
}

func (tg *testgroup) Clean() {
	tg.client1.Close()
	tg.client2.Close()
}
