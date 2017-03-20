package auth

import (
	"github.com/cosminrentea/gobbler/protocol"

	log "github.com/Sirupsen/logrus"

	"io/ioutil"
	"net/http"
	"net/url"
)

// RestAccessManager is a url for which the access is allowed or not.
type RestAccessManager string

// NewRestAccessManager returns a new RestAccessManager.
func NewRestAccessManager(url string) RestAccessManager {
	return RestAccessManager(url)
}

// IsAllowed is an implementation of the AccessManager interface.
// The boolean result is based on matching between the desired AccessType, the userId and the path.
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
		logger.WithError(err).WithField("module", "RestAccessManager").Warn("Write message failed")
		return false
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	if err != nil || resp.StatusCode != 200 {
		logger.WithError(err).WithField("httpCode", resp.StatusCode).Info("Error getting permission")
		logger.WithField("responseBody", responseBody).Debug("HTTP Response Body")
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
