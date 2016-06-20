package auth

import (
	"github.com/smancke/guble/protocol"

	log "github.com/Sirupsen/logrus"

	"io/ioutil"
	"net/http"
	"net/url"
)

type RestAccessManager string

func NewRestAccessManager(url string) RestAccessManager {
	return RestAccessManager(url)
}

func (ram RestAccessManager) IsAllowed(accessType AccessType, userId string, path protocol.Path) bool {

	u, _ := url.Parse(string(ram))
	q := u.Query()
	if accessType == READ {
		q.Set("type", "read")
	} else {
		q.Set("type", "write")
	}

	q.Set("userId", userId)
	q.Set("path", string(path))

	resp, err := http.DefaultClient.Get(u.String())

	if err != nil {
		logger.WithFields(log.Fields{
			"module": "RestAccessManager",
			"err":    err,
		}).Warn("Write message failed")

		return false
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	if err != nil || resp.StatusCode != 200 {
		logger.WithFields(log.Fields{
			"err":      err,
			"httpCode": resp.StatusCode,
		}).Info("Error getting permission")
		logger.WithFields(log.Fields{
			"responseBody": responseBody,
		}).Debug("HTTP Response Body")
		return false
	}
	logger.WithFields(log.Fields{
		"access_type":  accessType,
		"userId":       userId,
		"path":         path,
		"responseBody": string(responseBody),
	}).Debug("Access allowed")
	return "true" == string(responseBody)
}
