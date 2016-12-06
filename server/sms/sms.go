package sms

import "encoding/json"

type NexmoSms struct {
	ApiKey    string `json:"api_key"`
	ApiSecret string `json:"api_secret"`
	To        string `json:"to"`
	From      string `json:"from"`
	Text      string `json:"text"`
}

func (sms *NexmoSms) EncodeNexmoSms(apiKey, apiSecret string) ([]byte, error) {
	sms.ApiKey = apiKey
	sms.ApiSecret = apiSecret

	d, err := json.Marshal(&sms)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Could not encode sms as json")
		return nil, err
	}
	return d, nil
}
