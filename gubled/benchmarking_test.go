package gubled

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"fmt"
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/protocol"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type testgroup struct {
	t                *testing.T
	groupId          int
	addr             string
	done             chan bool
	messagesToSend   int
	client1, client2 client.Client
	topic            string
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

func TestThroughput(t *testing.T) {
	//defer enableDebugForMethod()()
	dir, _ := ioutil.TempDir("", "guble_benchmark_test")
	defer os.RemoveAll(dir)

	service := StartupService(Args{Listen: "localhost:0", KVBackend: "memory", MSBackend: "file", StoragePath: dir})
	defer func() {
		service.Stop()
	}()
	time.Sleep(time.Millisecond * 10)

	testgroupCount := 4
	messagesPerGroup := 100
	log.Printf("init the %v testgroups", testgroupCount)
	testgroups := make([]*testgroup, testgroupCount, testgroupCount)
	for i, _ := range testgroups {
		testgroups[i] = newTestgroup(t, i, service.WebServer().GetAddr(), messagesPerGroup)
	}

	// init test
	log.Print("init the testgroups")
	for i, _ := range testgroups {
		testgroups[i].Init()
	}

	defer func() {
		// cleanup tests
		log.Print("cleanup the testgroups")
		for i, _ := range testgroups {
			testgroups[i].Clean()
		}
	}()

	// start test
	log.Print("start the testgroups")
	start := time.Now()
	for i, _ := range testgroups {
		go testgroups[i].Start()
	}

	log.Print("wait for finishing")
	timeout := time.After(time.Second * 60)
	for i, test := range testgroups {
		//fmt.Printf("wating for test %v\n", i)
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

	end := time.Now()
	totalMessages := testgroupCount * messagesPerGroup
	throughput := float64(totalMessages) / end.Sub(start).Seconds()
	log.Printf("finished! Throughput: %v/sec (%v message in %v)", int(throughput), totalMessages, end.Sub(start))
}

func (test *testgroup) Init() {
	test.topic = fmt.Sprintf("/%v-foo", test.groupId)
	var err error
	location := "ws://" + test.addr + "/stream/user/xy"
	//location := "ws://gathermon.mancke.net:8080/stream/"
	//location := "ws://127.0.0.1:8080/stream/"
	test.client1, err = client.Open(location, "http://localhost/", 10, false)
	if err != nil {
		panic(err)
	}
	test.client2, err = client.Open(location, "http://localhost/", 10, false)
	if err != nil {
		panic(err)
	}

	test.expectStatusMessage(protocol.SUCCESS_CONNECTED, "You are connected to the server.")

	test.client1.Subscribe(test.topic)
	time.Sleep(time.Millisecond * 1)
	//test.expectStatusMessage(protocol.SUCCESS_SUBSCRIBED_TO, test.topic)
}

func (test *testgroup) expectStatusMessage(name string, arg string) {
	select {
	case notify := <-test.client1.StatusMessages():
		assert.Equal(test.t, name, notify.Name)
		assert.Equal(test.t, arg, notify.Arg)
	case <-time.After(time.Second * 1):
		test.t.Logf("[%v] no notification of type %s after 1 second", test.groupId, name)
		test.done <- false
		test.t.Fail()
		return
	}
}

func (test *testgroup) Start() {
	go func() {
		for i := 0; i < test.messagesToSend; i++ {
			body := fmt.Sprintf("Hallo-%v", i)
			test.client2.Send(test.topic, body, "")
		}
	}()

	for i := 0; i < test.messagesToSend; i++ {
		body := fmt.Sprintf("Hallo-%v", i)

		select {
		case msg := <-test.client1.Messages():
			assert.Equal(test.t, body, msg.BodyAsString())
			assert.Equal(test.t, test.topic, string(msg.Path))
		case msg := <-test.client1.Errors():
			test.t.Logf("[%v] received error: %v", test.groupId, msg)
			test.done <- false
			test.t.Fail()
			return
		case <-time.After(time.Second * 5):
			test.t.Logf("[%v] no message received for 5 seconds, expected message %v", test.groupId, i)
			test.done <- false
			test.t.Fail()
			return
		}
	}
	test.done <- true
}

func (test *testgroup) Clean() {
	test.client1.Close()
	test.client2.Close()
}
