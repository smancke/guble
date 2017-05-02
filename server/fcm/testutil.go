package fcm

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Bogh/gcm"
	"github.com/cosminrentea/gobbler/server/connector"
)

const (
	SuccessFCMResponse = `{
	   "multicast_id":3,
	   "success":1,
	   "failure":0,
	   "canonical_ids":0,
	   "results":[
	      {
	         "message_id":"da",
	         "registration_id":"rId",
	         "error":""
	      }
	   ]
	}`

	ErrorFCMResponse = `{
	   "multicast_id":3,
	   "success":0,
	   "failure":1,
       "error":"InvalidRegistration",
	   "canonical_ids":5,
	   "results":[
	      {
	         "message_id":"err",
	         "registration_id":"fcmCanonicalID",
	         "error":"InvalidRegistration"
	      }
	   ]
	}`
)

func NewSenderWithMock(gcmSender gcm.Sender) *sender {
	return &sender{gcmSender: gcmSender}
}

type FCMSender func(message *gcm.Message) (*gcm.Response, error)

func (fcms FCMSender) Send(message *gcm.Message) (*gcm.Response, error) {
	return fcms(message)
}

func CreateFcmSender(body string, doneC chan bool, to time.Duration) (connector.Sender, error) {
	response := new(gcm.Response)

	err := json.Unmarshal([]byte(body), response)
	if err != nil {
		return nil, err
	}

	return NewSenderWithMock(FCMSender(func(message *gcm.Message) (*gcm.Response, error) {
		defer func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("Recovered")
				}
			}()
			doneC <- true
		}()
		<-time.After(to)
		return response, nil
	})), nil
}
