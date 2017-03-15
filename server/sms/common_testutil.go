package sms

import (
	"encoding/json"
	"fmt"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"
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
		ClientReference:  "",
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
	return Config{
		Workers:  &worker,
		SMSTopic: &topic,
		Name:     "test_gateway",
		Schema:   SMSSchema,

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
		To:   "toNumber",
		From: "FromNUmber",
		Text: "body",
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
