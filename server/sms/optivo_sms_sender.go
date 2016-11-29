package gateway

import (
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"net/http"
	//"strconv"
	"bytes"
	"encoding/json"
	"strconv"
)

type NexmoSender struct {
	logger    *log.Entry
	ApiKey    string
	ApiSecret string
}

const (
	KEY    = "ce40b46d"
	SECRET = "153d2b2c72985370"
	URL = "https://rest.nexmo.com/sms/json?"
)

func NewNexmoSender(apiKey, ApiSecret string) (Sender, error) {
	return &NexmoSender{
		logger:    logger.WithField("name", "optivoSender"),
		ApiKey:    apiKey,
		ApiSecret: ApiSecret,
	}, nil
}

func (os *NexmoSender) Send(msg *protocol.Message) error {
	os.logger.WithFields(log.Fields{
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

func SendSms() error {

	sms := NexmoSms{
		ApiKey:    KEY,
		ApiSecret: SECRET,
		To:        "40746278186",
		From:      "NEXMO TEST",
		SmsBody:   "THIS IS TEST",
	}

	d, err := json.Marshal(&sms)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, URL, bytes.NewBuffer(d))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Content-Length", strconv.Itoa(len(d)))

	client := &http.Client{}
	resp, err := client.Do(req)
	log.WithField("ERR", err).WithField("resp", resp).Info("Error")
	if err != nil {
		return err
	}
	return nil
}
