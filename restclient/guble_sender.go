package restclient

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type GubleSender struct {
	Endpoint string
}

// Check returns `true` if the guble endpoint is reachable through a HEAD request, or `false` otherwise
func (gs GubleSender) Check() bool {
	req, err := http.NewRequest(http.MethodHead, gs.Endpoint, nil)
	if err != nil {
		logger.WithError(err).Error("error creating request url")
		return false
	}
	c := &http.Client{}
	res, err := c.Do(req)
	if err != nil {
		logger.WithError(err).Error("error reaching guble endpoint")
		return false
	}
	return res.StatusCode == http.StatusOK
}

func (gs GubleSender) Send(topic string, body []byte) error {
	url := fmt.Sprintf("%s%s?userId=%s",
		strings.TrimPrefix(gs.Endpoint, "/"),
		topic,
		"samsa")
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		logger.WithFields(log.Fields{
			"header": resp.Header,
			"code":   resp.StatusCode,
			"status": resp.Status,
		}).Error("Guble response error")
		return fmt.Errorf("Error code returned from guble: %d", resp.StatusCode)
	}
	return nil
}
