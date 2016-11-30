package gateway

import (
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"net/http"
	"bytes"
	"encoding/json"
	"strconv"
	"io/ioutil"
)

type NexmoSender struct {
	logger    *log.Entry
	ApiKey    string
	ApiSecret string
}

const (
	KEY    = "ce40b46d"
	SECRET = "153d2b2c72985370"
	URL    = "https://rest.nexmo.com/sms/json?"
)

func NewNexmoSender(apiKey, apiSecret string) (*NexmoSender, error) {
	return &NexmoSender{
		logger:    logger.WithField("name", "optivoSender"),
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
	}, nil
}

func (ns *NexmoSender) Send(msg *protocol.Message) error {
	ns.logger.WithFields(log.Fields{
		"ID":   msg.ID,
		"body": msg.Body,
	}).Info("Sending Message  to Optivo")
	return nil
}

type NexmoSms struct {
	ApiKey    string `json:"api_key"`
	ApiSecret string `json:"api_secret"`
	To        string `json:"to"`
	From      string `json:"from"`
	SmsBody   string `json:"text"`
}

type ResponseCode int

type VerifyCheckResponse struct {
	Status    ResponseCode `json:"status,string"`
	EventID   string       `json:"event_id"`
	Price     string       `json:"price"`
	Currency  string       `json:"currency"`
	ErrorText string       `json:"error_text"`
}


const (
	ResponseSuccess ResponseCode = iota
	ResponseThrottled
	ResponseMissingParams
	ResponseInvalidParams
	ResponseInvalidCredentials
	ResponseInternalError
	ResponseInvalidMessage
	ResponseNumberBarred
	ResponsePartnerAcctBarred
	ResponsePartnerQuotaExceeded
	ResponseRESTNotEnabled
	ResponseMessageTooLong
	ResponseCommunicationFailed
	ResponseInvalidSignature
	ResponseInvalidSenderAddress
	ResponseInvalidTTL
	ResponseFacilityNotAllowed
	ResponseInvalidMessageClass
)

var nexmoResponseCodeMap = map[ResponseCode]string{
	ResponseSuccess:              "Success",
	ResponseThrottled:            "Throttled",
	ResponseMissingParams:        "Missing params",
	ResponseInvalidParams:        "Invalid params",
	ResponseInvalidCredentials:   "Invalid credentials",
	ResponseInternalError:        "Internal error",
	ResponseInvalidMessage:       "Invalid message",
	ResponseNumberBarred:         "Number barred",
	ResponsePartnerAcctBarred:    "Partner account barred",
	ResponsePartnerQuotaExceeded: "Partner quota exceeded",
	ResponseRESTNotEnabled:       "Account not enabled for REST",
	ResponseMessageTooLong:       "Message too long",
	ResponseCommunicationFailed:  "Communication failed",
	ResponseInvalidSignature:     "Invalid signature",
	ResponseInvalidSenderAddress: "Invalid sender address",
	ResponseInvalidTTL:           "Invalid TTL",
	ResponseFacilityNotAllowed:   "Facility not allowed",
	ResponseInvalidMessageClass:  "Invalid message class",
}

func (c ResponseCode) String() string {
	return nexmoResponseCodeMap[c]
}

// NexmoMessageReport is the "status report" for a single SMS sent via the Nexmo API
type NexmoMessageReport struct {
	Status           ResponseCode `json:"status,string"`
	MessageID        string       `json:"message-id"`
	To               string       `json:"to"`
	ClientReference  string       `json:"client-ref"`
	RemainingBalance string       `json:"remaining-balance"`
	MessagePrice     string       `json:"message-price"`
	Network          string       `json:"network"`
	ErrorText        string       `json:"error-text"`
}

type NexmoMessageResponse struct {
	MessageCount int             `json:"message-count,string"`
	Messages     []NexmoMessageReport `json:"messages"`
}

func EncodeNexmoSms(toPhoneNumber, from, body string) ([]byte, error) {

	sms := NexmoSms{
		ApiKey:    KEY,
		ApiSecret: SECRET,
		To:        toPhoneNumber,
		From:      from,
		SmsBody:   body,
	}

	d, err := json.Marshal(&sms)
	if err != nil {
		log.WithField("error", err.Error()).Error("Could not encode sms as json")
		return nil, err
	}
	return d, nil

}

func  (ns *NexmoSender)SendSms( toPhoneNumber, from, body string)(*NexmoMessageResponse, error) {
	smsEncoded ,err := EncodeNexmoSms(toPhoneNumber,from,body)
	if err != nil {
		log.WithField("error", err.Error()).Error("Error encoding sms")
		return nil,err
	}

	req, err := http.NewRequest(http.MethodPost, URL, bytes.NewBuffer(smsEncoded))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Content-Length", strconv.Itoa(len(smsEncoded)))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithField("error", err.Error()).Error("Error doing the request to sms endpoint")
		return nil,err
	}

	defer resp.Body.Close()

	var messageResponse *NexmoMessageResponse
	respBody, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(respBody, &messageResponse)
	if err != nil {
		log.WithField("error", err.Error()).Error("Error decoding the response from sms endpoint")
		return  nil,err
	}
	log.WithField("messageResponse", messageResponse).Debug("Actual response was")

	return  messageResponse,nil
}
