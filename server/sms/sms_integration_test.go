package sms

import (
	"encoding/json"
	"fmt"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

func Test_NexmoHTTPError(t *testing.T) {
	a := assert.New(t)

	port := createRandomPort(7000, 9000)
	URL = "http://127.0.0.1" + port

	countCh := make(chan bool, 0)
	go dummyNexmoEndpointWithHandlerFunc(t, countCh, port, noResponseFromNexmoHandler)
	kvStore, f := createKVStore(t, "/guble_sms_nexmo_error")
	defer os.Remove(f)

	gw := createGateway(t, kvStore)

	msg := encodeProtocolMessage(t, 2)

	err := gw.route.Deliver(&msg, false)
	a.NoError(err)

	readServedResponses(t, 3, countCh)
	//await for gateway to write lastIDSent in kvstore.
	time.Sleep(timeInterval)
	a.Equal(msg.ID, gw.LastIDSent, "Retry failed.Last id  sent should be msgId")

	stopGateway(t, gw)
}

func Test_NexmoInvalidSenderError(t *testing.T) {
	//defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	port := createRandomPort(7000, 9000)
	URL = "http://127.0.0.1" + port

	countCh := make(chan bool, 0)
	go dummyNexmoEndpointWithHandlerFunc(t, countCh, port, invalidSenderNexmoHandler)
	kvStore, f := createKVStore(t, "/guble_sms_nexmo_invalid_sender_error")
	defer os.Remove(f)

	gw := createGateway(t, kvStore)

	msg := encodeProtocolMessage(t, 2)
	err := gw.route.Deliver(&msg, false)
	a.NoError(err)

	//Only one try should be made by sender.
	readServedResponses(t, 1, countCh)

	//await for gateway to write lastIDSent in kvstore.
	time.Sleep(timeInterval)
	a.Equal(msg.ID, gw.LastIDSent, "No Retry needed.Last id  sent should be msgId")

	stopGateway(t, gw)
}

func Test_NexmoMultipleErrorsFollowedBySuccess(t *testing.T) {
	a := assert.New(t)

	port := createRandomPort(7000, 9000)
	URL = "http://127.0.0.1" + port

	countCh := make(chan bool, 0)
	go dummyNexmoEndpointWithHandlerFunc(t, countCh, port, multipleErrorsFollowedBySuccessNexmoHandler)
	kvStore, f := createKVStore(t, "/guble_sms_nexmo_multiple_sender_error")
	defer os.Remove(f)

	gw := createGateway(t, kvStore)

	msg := encodeProtocolMessage(t, 2)
	err := gw.route.Deliver(&msg, false)
	a.NoError(err)

	// 3 retries should be made.
	readServedResponses(t, 3, countCh)
	//await for gateway to write lastIDSent in kvstore.
	time.Sleep(timeInterval)
	a.Equal(msg.ID, gw.LastIDSent, "No Retry needed.Last id  sent should be msgId")

	stopGateway(t, gw)
}

func Test_NexmoResponseCodeError(t *testing.T) {
	//defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	port := createRandomPort(7000, 9000)
	URL = "http://127.0.0.1" + port

	countCh := make(chan bool, 0)
	go dummyNexmoEndpointWithHandlerFunc(t, countCh, port, responseInternalErrorNexmoHandler)
	kvStore, f := createKVStore(t, "/guble_sms_nexmo_responde_code_error")
	defer os.Remove(f)

	gw := createGateway(t, kvStore)

	msg := encodeProtocolMessage(t, 2)
	err := gw.route.Deliver(&msg, false)
	a.NoError(err)
	//3 retries should be made
	readServedResponses(t, 3, countCh)
	time.Sleep(timeInterval)
	a.Equal(msg.ID, gw.LastIDSent, "No Retry needed.Last id  sent should be msgId")

	stopGateway(t, gw)
}

func Test_WrongEncodedSmsInRouterMessage(t *testing.T) {
	a := assert.New(t)

	kvStore, f := createKVStore(t, "/guble_sms_nexmo_responde_code_error")
	defer os.Remove(f)

	gw := createGateway(t, kvStore)

	msg := encodeUnmarshallableProtocolMessage(2)
	err := gw.route.Deliver(&msg, false)
	a.NoError(err)

	time.Sleep(timeInterval)
	a.Equal(msg.ID, gw.LastIDSent, "No Retry needed.Last id  sent should be msgId")

	stopGateway(t, gw)
}

func Test_GatewaySanity(t *testing.T) {
	//defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	port := createRandomPort(7000, 9000)
	URL = "http://127.0.0.1" + port

	countCh := make(chan bool, 0)
	go dummyNexmoEndpointWithHandlerFunc(t, countCh, port, multipleMessageNexmoHandler)
	kvStore, f := createKVStore(t, "/guble_sms_nexmo_responde_code_error")
	defer os.Remove(f)

	gw := createGateway(t, kvStore)

	//deliver message with ID=2 should be success
	msg := encodeProtocolMessage(t, 2)
	err := gw.route.Deliver(&msg, false)
	a.NoError(err)

	//only one request should be made to server
	readServedResponses(t, 1, countCh)
	time.Sleep(timeInterval)
	a.Equal(msg.ID, gw.LastIDSent, fmt.Sprintf("Sucess.No Retry needed.Last id  sent should be %d", msg.ID))

	//deliver message with ID=5 should have invalid sender response.Only one request should be made
	invalidSenderMsg := encodeProtocolMessage(t, 5)
	err = gw.route.Deliver(&invalidSenderMsg, false)
	a.NoError(err)
	//only one request should be made to server
	readServedResponses(t, 1, countCh)
	time.Sleep(timeInterval)
	a.Equal(invalidSenderMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Invalid Sender.No Retry needed.Last id  sent should be %d", invalidSenderMsg.ID))

	//deliver message with ID=6 should be success
	successMsg := encodeProtocolMessage(t, 6)
	err = gw.route.Deliver(&successMsg, false)
	a.NoError(err)
	//only one request should be made to server
	readServedResponses(t, 1, countCh)
	time.Sleep(timeInterval)
	a.Equal(successMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Success.No Retry needed.Last id  sent should be %d", successMsg.ID))

	//deliver message with ID=8 should made 3 retries(server will always close the connection) and fail.LastID sent should be 8
	closedConnectionMsg := encodeProtocolMessage(t, 8)
	err = gw.route.Deliver(&closedConnectionMsg, false)
	a.NoError(err)
	//3 request should be made to server by retries
	readServedResponses(t, 3, countCh)
	time.Sleep(timeInterval)
	a.Equal(closedConnectionMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Retry failed.No retries should be made further.Last id  sent should be %d", closedConnectionMsg.ID))

	//deliver message with ID=10 should be success
	successMsg = encodeProtocolMessage(t, 10)
	err = gw.route.Deliver(&successMsg, false)
	a.NoError(err)
	//3 request should be made to server by retries
	readServedResponses(t, 1, countCh)
	time.Sleep(timeInterval)
	a.Equal(successMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Success.No Retry needed.Last id  sent should be %d", successMsg.ID))

	//deliver message with ID=11 should made 3 retries(server will return  NexmoResponse statusCode not ResponseOk) and fail.LastID sent should be 11
	randomNexmoErrMsg := encodeProtocolMessage(t, 11)
	err = gw.route.Deliver(&randomNexmoErrMsg, false)
	//3 request should be made to server by retries
	readServedResponses(t, 3, countCh)
	time.Sleep(timeInterval)
	a.Equal(randomNexmoErrMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Retry failed.No retries should be made further.Last id  sent should be %d", randomNexmoErrMsg.ID))

	//deliver message with ID=13 should made 3 retries(server will respond with a message that does not have the NexmoResponse Structure) and fail.LastID sent should be 13
	undecodableNexmoResponseMsg := encodeProtocolMessage(t, 13)
	err = gw.route.Deliver(&undecodableNexmoResponseMsg, false)
	a.NoError(err)
	//3 request should be made to server by retries
	readServedResponses(t, 3, countCh)
	time.Sleep(timeInterval)
	a.Equal(undecodableNexmoResponseMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Retry failed.No retries should be made further.Last id  sent should be %d", undecodableNexmoResponseMsg.ID))

	//deliver message with ID=14 should made 3 retries(Server will return a wrong MessageCount) and fail.LastID sent should be 14
	wrongMessageCountMsg := encodeProtocolMessage(t, 14)
	err = gw.route.Deliver(&wrongMessageCountMsg, false)
	a.NoError(err)
	//3 request should be made to server by retries
	readServedResponses(t, 3, countCh)
	time.Sleep(timeInterval)
	a.Equal(wrongMessageCountMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Retry failed.No retries should be made further.Last id  sent should be %d", wrongMessageCountMsg.ID))

	//deliver message with ID=16 should be success
	successMsg = encodeProtocolMessage(t, 16)
	err = gw.route.Deliver(&successMsg, false)
	a.NoError(err)
	//only one request should be made to server
	readServedResponses(t, 1, countCh)
	time.Sleep(timeInterval)
	a.Equal(successMsg.ID, gw.LastIDSent,
		fmt.Sprintf("Success.No Retry needed.Last id  sent should be %d", successMsg.ID))

	//now close the route channel.Restart loop should happen.
	err = gw.route.Close()
	a.Equal(router.ErrInvalidRoute, err)
	time.Sleep(3 * timeInterval)
	a.Equal(successMsg.ID, gw.LastIDSent, "LastID read should be same after restart.")
	stopGateway(t, gw)
}

func noResponseFromNexmoHandler(t *testing.T, ch chan bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := assert.New(t)
		sentSms := decodeSMSMessage(t, r)
		a.Equal("body", sentSms.Text)
		ch <- true
	}
}

func multipleMessageNexmoHandler(t *testing.T, countCh chan bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := assert.New(t)
		sentSms := decodeSMSMessage(t, r)
		a.Equal("body", sentSms.Text)

		currentID, err := strconv.Atoi(sentSms.From)
		if err != nil {
			a.FailNow("Could not read message id.")
		}
		//msgID with 2 will  be marked as correct.
		if currentID == 2 {
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseSuccess, 1)
			writeNexmoResponse(nexmoResponse, t, w)
		} else if currentID == 5 { //msgID=5 will received a wrong invalid sender
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseInvalidSenderAddress, 1)
			writeNexmoResponse(nexmoResponse, t, w)
		} else if currentID == 6 { //msgID=6 will  be marked as correct.
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseSuccess, 1)
			writeNexmoResponse(nexmoResponse, t, w)
		} else if currentID == 8 { // msgID=8 connection will be closed  as in PN-307 involving a sender recreation.
			closeClientConnection(w, t)
		} else if currentID == 10 { // msgID=10  will be marked as correct.
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseSuccess, 1)
			writeNexmoResponse(nexmoResponse, t, w)
		} else if currentID == 11 { //msgID will have a random error from Nexmo list
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponsePartnerAcctBarred, 1)
			writeNexmoResponse(nexmoResponse, t, w)
		} else if currentID == 13 { // msgID= 13 will have a body that can not be decoded
			w.Write([]byte("This should not be decoded."))
		} else if currentID == 14 { // msgID=14  will have a wrong messageCount
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseSuccess, 4)
			writeNexmoResponse(nexmoResponse, t, w)
		} else if currentID == 16 { //msgID=16 will be marked as correct.
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseSuccess, 1)
			writeNexmoResponse(nexmoResponse, t, w)
		}
		countCh <- true
	}
}

func invalidSenderNexmoHandler(t *testing.T, countCh chan bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := assert.New(t)
		sentSms := decodeSMSMessage(t, r)
		a.Equal("body", sentSms.Text)

		nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseInvalidSenderAddress, 1)
		writeNexmoResponse(nexmoResponse, t, w)
		countCh <- true

	}
}

func multipleErrorsFollowedBySuccessNexmoHandler(t *testing.T, countCh chan bool) http.HandlerFunc {
	//expect 3 request to be made .
	noOfReq := 3
	return func(w http.ResponseWriter, r *http.Request) {
		a := assert.New(t)
		sentSms := decodeSMSMessage(t, r)
		a.Equal("body", sentSms.Text)
		noOfReq--

		//on the first try, hijack the net.connection to forcibly  close the connection as per PN-307.
		if noOfReq == 2 {
			logger.Info("Closing request from server side by hijacking.")
			closeClientConnection(w, t)
		} else if noOfReq == 1 { //on the second try write an answer that can not be decoded.
			logger.Info("Serving a wrong response to request")
			w.Write([]byte("This should not be decoded."))
		} else { //on  the last retry write a SuccesResponse.
			logger.Info("Serving correct response")
			nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseSuccess, 1)
			writeNexmoResponse(nexmoResponse, t, w)
		}
		countCh <- true
	}
}
func closeClientConnection(w http.ResponseWriter, t *testing.T) {
	a := assert.New(t)
	hj, ok := w.(http.Hijacker)
	if !ok {
		a.FailNow("Failed to obtain hijacker.")
	}
	con, _, err := hj.Hijack()
	if err != nil {
		a.FailNow("Hijack failed.")
	}
	err = con.Close()
	if err != nil {
		a.FailNow(" Forced connection closing failed.")
	}
}

func responseInternalErrorNexmoHandler(t *testing.T, countCh chan bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := assert.New(t)
		sentSms := decodeSMSMessage(t, r)
		a.Equal("body", sentSms.Text)

		nexmoResponse := composeNexmoMessageResponse(sentSms, ResponseInternalError, 1)

		writeNexmoResponse(nexmoResponse, t, w)
		countCh <- true

	}
}

func dummyNexmoEndpointWithHandlerFunc(t *testing.T, countCh chan bool, port string, handler func(t *testing.T, countCh chan bool) http.HandlerFunc) {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", handler(t, countCh))
	logger.Info("Started Http server on port")
	http.ListenAndServe(port, serveMux)
}

func decodeSMSMessage(t *testing.T, r *http.Request) NexmoSms {
	a := assert.New(t)
	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)
	var sentSms NexmoSms
	err := json.Unmarshal(body, &sentSms)
	if err != nil {
		a.FailNow("Could not decode sender sms.")
	}
	return sentSms
}

func writeNexmoResponse(nexmoResponse *NexmoMessageResponse, t *testing.T, w http.ResponseWriter) {
	a := assert.New(t)
	jData, err := json.Marshal(nexmoResponse)
	if err != nil {
		a.FailNow("Nexmo Response encoding failed.")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jData)
}
