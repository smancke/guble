package restclient

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"io/ioutil"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/cosminrentea/gobbler/server/rest"
)

const (
	correlationIDLiteral = "correlationID"
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

func (gs gubleSender) GetSubscribers(topic string) ([]byte, error) {
	logger.WithField("topic", topic).Info("GetSubscribers called")
	body := make([]byte, 0)
	request, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("%s/subscribers/%s", gs.Endpoint, trimPrefixSlash(topic)),
		bytes.NewReader(body),
	)
	logger.WithField("url", fmt.Sprintf("%s/subscribers/%s", gs.Endpoint, topic))
	if err != nil {
		return nil, err
	}

	response, err := gs.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger.WithFields(log.Fields{
			"header": response.Header,
			"code":   response.StatusCode,
			"status": response.Status,
		}).Error("Guble response error")
		return nil, fmt.Errorf("Error code returned from guble: %d", response.StatusCode)
	}

	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	logger.WithFields(log.Fields{
		"header": response.Header,
		"code":   response.StatusCode,
		"body":   string(content),
	}).Debug("Guble response")
	return content, nil
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
	logger.WithFields(log.Fields{
		"topic":  topic,
		"body":   body,
		"userID": userID,
		"params": params,
	}).Debug("Sending guble message")
	request, err := http.NewRequest(http.MethodPost, getURL(gs.Endpoint, topic, userID, params), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Add(rest.XHeaderPrefix+"correlation-id", params[correlationIDLiteral])
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

func getURL(endpoint, topic, userID string, params map[string]string) string {
	uv := url.Values{}
	uv.Add("userId", userID)
	if params != nil {
		for k, v := range params {
			if k != "" {
				uv.Add(k, v)
			}
		}
	}
	return fmt.Sprintf("%s/%s?%s", endpoint, topic, uv.Encode())
}

func trimPrefixSlash(topic string) string {
	if strings.HasPrefix(topic, "/") {
		return strings.TrimPrefix(topic, "/")
	}
	return topic
}
