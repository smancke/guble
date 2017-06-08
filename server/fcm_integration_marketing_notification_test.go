package server

import (
	"net/http"
	"testing"
	"time"

	"fmt"

	"bytes"
	"io/ioutil"

	"github.com/cosminrentea/gobbler/restclient"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/server/fcm"
	"github.com/cosminrentea/gobbler/server/service"
	gobblertestutil "github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
)

func Test_SendMarketingNotification(t *testing.T) {
	//gobblertestutil.SkipIfDisabled(t)
	//defer gobblertestutil.SkipIfShort(t)
	defer gobblertestutil.EnableDebugForMethod()()
	defer gobblertestutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(t)

	restClient := restclient.New("http://localhost:8080/api/message")

	*Config.HttpListen = "localhost:8080"
	*Config.KVS = "memory"
	*Config.MS = "memory"

	*Config.FCM.Enabled = true
	*Config.FCM.APIKey = "WILL BE OVERWRITTEN"
	*Config.FCM.Workers = 1
	*Config.FCM.Prefix = "/fcm/"
	*Config.APNS.Enabled = false
	*Config.Cluster.NodeID = 0

	receiveC := make(chan bool)

	s := StartService()
	a.NotNil(s)
	time.Sleep(time.Millisecond * 400)

	var fcmConn connector.ResponsiveConnector
	var ok bool
	for _, iface := range s.ModulesSortedByStartOrder() {
		fcmConn, ok = iface.(connector.ResponsiveConnector)
		if ok {
			break
		}
	}
	a.True(ok, "There should be a module of type FCMConnector")

	// add a high timeout so the messages are processed slow
	sender, err := fcm.CreateFcmSender(fcm.SuccessFCMResponse, receiveC, 10*time.Millisecond)
	a.NoError(err)
	fcmConn.SetSender(sender)

	//subscribe a client

	subcribe(s, t)
	topic := "marketing_notifications_general"
	body := []byte(`{"to":"","data":{"deep_link":"rewe://angebote","notification_body":"Die größte Sonderangebot!","notification_title":"REWE","time":"2016-09-08T08:25:13+02:00","type":"general"}`)
	userID := "samsa"
	params := map[string]string{
		"filterConnector": "fcm",
		"correlationID":   "correlation-id",
	}
	err = restClient.Send(topic, body, userID, params)
	a.NoError(err)

	counter := 0

	select {
	case <-receiveC:
		counter++
	case <-time.After(timeoutForOneMessage):
		a.Fail("Initial FCM message not received")
	}
	a.Equal(1, counter, "One fcm message should have been received")

	err = s.Stop()
	//for ensuring the stop is done correctly.
	time.Sleep(250 * time.Millisecond)
	a.NoError(err)

}
func subcribe(s *service.Service, t *testing.T) {
	a := assert.New(t)
	topic := "marketing_notifications_general"
	url := fmt.Sprintf("http://%s/fcm/%s/%s/%s", s.WebServer().GetAddr(), "samsa", "1337", topic)
	response, errPost := http.Post(
		url,
		"text/plain",
		bytes.NewBufferString(""),
	)
	logger.WithField("url", url).Debug("subscribe")
	a.NoError(errPost)
	a.Equal(response.StatusCode, 200)
	body, errReadAll := ioutil.ReadAll(response.Body)
	a.NoError(errReadAll)
	a.Equal(fmt.Sprintf(`{"subscribed":"/%s"}`, topic), string(body))
}
