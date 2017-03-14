package sms

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cosminrentea/gobbler/protocol"
)

var (
	URL                = "https://rest.nexmo.com/sms/json?"
	MaxIdleConnections = 100
	RequestTimeout     = 500 * time.Millisecond
)

type ResponseCode int

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
	ResponseUnused
	ResponseRESTNotEnabled
	ResponseMessageTooLong
	ResponseCommunicationFailed
	ResponseInvalidSignature
	ResponseInvalidSenderAddress
	ResponseInvalidTTL
	ResponseFacilityNotAllowed
	ResponseInvalidMessageClass
)

var (
	ErrNoSMSSent                 = errors.New("No sms was sent to Nexmo")
	ErrIncompleteSMSSent         = errors.New("Nexmo sms was only partial delivered.One or more part returned an error")
	ErrSMSResponseDecodingFailed = errors.New("Nexmo response decoding failed.")
	ErrNoRetry                   = errors.New("SMS failed. No retrying.")
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
	MessageCount int                  `json:"message-count,string"`
	Messages     []NexmoMessageReport `json:"messages"`
}

func (nm NexmoMessageResponse) Check() error {
	if nm.MessageCount == 0 {
		return ErrNoSMSSent
	}
	for i := 0; i < nm.MessageCount; i++ {
		if nm.Messages[i].Status != ResponseSuccess {
			logger.WithField("status", nm.Messages[i].Status).
				WithField("error", nm.Messages[i].ErrorText).
				Error("Error received from Nexmo")

			if nm.Messages[i].Status == ResponseInvalidSenderAddress {
				return nil
			}

			return ErrIncompleteSMSSent
		}
	}
	return nil
}

type NexmoSender struct {
	logger    *log.Entry
	ApiKey    string
	ApiSecret string

	httpClient *http.Client
}

func NewNexmoSender(apiKey, apiSecret string) (*NexmoSender, error) {
	ns := &NexmoSender{
		logger:    logger.WithField("name", "nexmoSender"),
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
	}
	ns.createHttpClient()
	return ns, nil
}

func (ns *NexmoSender) Send(msg *protocol.Message) error {
	nexmoSMS := new(NexmoSms)
	err := json.Unmarshal(msg.Body, nexmoSMS)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Could not decode message body to send to nexmo")
		return err
	}
	nexmoSMSResponse, err := ns.sendSms(nexmoSMS)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Could not decode nexmo response message body")
		return err
	}
	logger.WithField("response", nexmoSMSResponse).Info("Decoded nexmo response")

	return nexmoSMSResponse.Check()
}

func (ns *NexmoSender) sendSms(sms *NexmoSms) (*NexmoMessageResponse, error) {
	// log before encoding
	logger.WithField("sms_details", sms).Info("sendSms")

	smsEncoded, err := sms.EncodeNexmoSms(ns.ApiKey, ns.ApiSecret)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Error encoding sms")
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, URL, bytes.NewBuffer(smsEncoded))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Content-Length", strconv.Itoa(len(smsEncoded)))

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Error doing the request to nexmo endpoint")
		ns.createHttpClient()
		mTotalSendErrors.Add(1)
		return nil, ErrNoSMSSent
	}
	defer resp.Body.Close()

	var messageResponse *NexmoMessageResponse
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Error reading the nexmo body response")
		mTotalResponseInternalErrors.Add(1)
		return nil, ErrSMSResponseDecodingFailed
	}

	err = json.Unmarshal(respBody, &messageResponse)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Error decoding the response from nexmo endpoint")
		mTotalResponseInternalErrors.Add(1)
		return nil, ErrSMSResponseDecodingFailed
	}
	logger.WithField("messageResponse", messageResponse).Info("Actual nexmo response")

	return messageResponse, nil
}

func (ns *NexmoSender) createHttpClient() {
	logger.Info("Recreating HTTP client for nexmo sender")
	ns.httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: MaxIdleConnections,
		},
		Timeout: RequestTimeout,
	}
}
