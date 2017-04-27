package sms

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/store/dummystore"
	"github.com/stretchr/testify/assert"
)

var (
	timeInterval = 100 * time.Millisecond
	KEY          = "ce40b46d"
	SECRET       = "153d2b2c72985370"
)

func tempFilename(name string) string {
	file, err := ioutil.TempFile("/tmp", name)
	if err != nil {
		panic(err)
	}
	file.Close()
	return file.Name()
}

func createRandomPort(min, max int) string {
	rand.Seed(time.Now().UnixNano())
	port := rand.Intn(max-min) + min
	return fmt.Sprintf(":%d", port)
}

func composeNexmoMessageResponse(sms NexmoSms, code ResponseCode, messageCount int) *NexmoMessageResponse {
	nexmoResponse := new(NexmoMessageResponse)
	nexmoResponse.MessageCount = messageCount
	response := NexmoMessageReport{
		Status:           code,
		MessageID:        "msgID",
		To:               sms.To,
		ClientReference:  "ref",
		RemainingBalance: "2",
		MessagePrice:     "0.005",
		Network:          "TELEKOM",
		ErrorText:        "",
	}
	nexmoResponse.Messages = []NexmoMessageReport{response}
	return nexmoResponse
}

func createConfig() Config {
	topic := "/sms"
	worker := 1
	intervalMetrics := true
	toggleable := false
	return Config{
		Workers:    &worker,
		SMSTopic:   &topic,
		Name:       "test_gateway",
		Schema:     SMSSchema,
		Toggleable: &toggleable,

		IntervalMetrics: &intervalMetrics,
	}
}

func createKVStore(t *testing.T, filename string) (kvstore.KVStore, string) {
	a := assert.New(t)
	f := tempFilename(filename)
	//defer os.Remove(f)

	//create a KVStore with sqlite3
	kvStore := kvstore.NewSqliteKVStore(f, true)
	err := kvStore.Open()
	if err != nil {
		a.FailNow("KVStore could not be opened.")
	}
	a.NotNil(kvStore)
	return kvStore, f
}

func encodeProtocolMessage(t *testing.T, ID int) protocol.Message {
	a := assert.New(t)
	sms := NexmoSms{
		To:        "toNumber",
		From:      fmt.Sprintf("%d", ID),
		Text:      "body",
		ClientRef: "ref",
	}
	d, err := json.Marshal(&sms)
	if err != nil {
		a.FailNow("Could not obtain message")
	}

	msg := protocol.Message{
		Path:          protocol.Path(SMSDefaultTopic),
		UserID:        "samsa",
		ApplicationID: "sms",
		ID:            uint64(ID),
		Body:          d,
	}
	return msg
}

func encodeUnmarshallableProtocolMessage(ID int) protocol.Message {
	msg := protocol.Message{
		Path:          protocol.Path(SMSDefaultTopic),
		UserID:        "samsa",
		ApplicationID: "sms",
		ID:            uint64(ID),
		Body:          []byte("undecodable"),
	}
	return msg
}

func createGateway(t *testing.T, kvStore kvstore.KVStore) *gateway {
	a := assert.New(t)

	msgStore := dummystore.New(kvStore)
	unstartedRouter := router.New(msgStore, kvStore, nil)
	gw, err := New(unstartedRouter, createNexmoSender(t), createConfig())
	a.NoError(err)
	err = gw.Start()
	if err != nil {
		a.FailNow("Sms gateway could not be started.")
	}

	return gw
}

func stopGateway(t *testing.T, gw *gateway) {
	a := assert.New(t)
	err := gw.Stop()
	time.Sleep(timeInterval)
	a.NoError(err)
}

func createNexmoSender(t *testing.T) Sender {
	a := assert.New(t)
	nexmoSender, err := NewNexmoSender(KEY, SECRET, nil, "")
	if err != nil {
		a.FailNow("Nexmo sender could not be created.")
	}
	return nexmoSender
}

func readServedResponses(t *testing.T, expectedRequestNo int, countCh chan bool) {
	a := assert.New(t)
	for i := 0; i < expectedRequestNo; i++ {
		select {
		case <-countCh:
			logger.WithField("request_number", i).Info("Read from channel")
		case <-time.After(1 * time.Minute):
			a.FailNow("Timeout expired.")
			return
		}
	}
}
