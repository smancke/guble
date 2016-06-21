package auth

import (
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"io/ioutil"
	"net/http"
	"net/url"
)

// AccessType permission required by the user
type AccessType int

const (
	// READ permission
	READ AccessType = iota

	// WRITE permission
	WRITE
)

var logger = log.WithFields(log.Fields{
	"service":          "guble",
	"application_type": "service",
	"module":           "accessManager",
	"env":              "TBD"})

// AccessManager interface allows to provide a custom authentication mechanism
type AccessManager interface {
	IsAllowed(accessType AccessType, userId string, path protocol.Path) bool
}

//AllowAllAccessManager  is a dummy implementation that grants access for everything
type AllowAllAccessManager bool

func NewAllowAllAccessManager(allowAll bool) AllowAllAccessManager {
	return AllowAllAccessManager(allowAll)
}

func (am AllowAllAccessManager) IsAllowed(accessType AccessType, userId string, path protocol.Path) bool {
	return bool(am)
}

type RestAccessManager string

func NewRestAccessManager(url string) RestAccessManager {
	return RestAccessManager(url)
}

func (am RestAccessManager) IsAllowed(accessType AccessType, userId string, path protocol.Path) bool {

	u, _ := url.Parse(string(am))
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
		}).Debug("HTTP Response  MSG Body")

		return false
	}

	logger.WithFields(log.Fields{
		"access_type":  accessType,
		"userId":       userId,
		"path":         path,
		"responseBody": string(responseBody),
	}).Debug("Is allowed for")

	return "true" == string(responseBody)

}
