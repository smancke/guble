package restclient

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type gubleSender struct {
	Endpoint   string
	httpClient *http.Client
}

// New returns a new Sender.
func New(endpoint string) Sender {
	return &gubleSender{
		Endpoint:   endpoint,
		httpClient: &http.Client{},
	}
}

func (gs gubleSender) Check() bool {
	request, err := http.NewRequest(http.MethodHead, gs.Endpoint, nil)
	if err != nil {
		logger.WithError(err).Error("error creating request url")
		return false
	}
	response, err := gs.httpClient.Do(request)
	if err != nil {
		logger.WithError(err).Error("error reaching guble server endpoint")
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func (gs gubleSender) Send(topic string, body []byte, userID string, params map[string]string) error {
	url := fmt.Sprintf("%s/%s?userId=%s",
		strings.TrimPrefix(gs.Endpoint, "/"), topic, userID)
	if params != nil {
		for k, v := range params {
			url = url + "&" + k + "=" + v
		}
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	response, err := gs.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger.WithFields(log.Fields{
			"header": response.Header,
			"code":   response.StatusCode,
			"status": response.Status,
		}).Error("Guble response error")
		return fmt.Errorf("Error code returned from guble: %d", response.StatusCode)
	}
	return nil
}
