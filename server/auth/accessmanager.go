package auth

import (
	"github.com/smancke/guble/guble"
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

// AccessManager interface allows to provide a custom authentication mechanism
type AccessManager interface {
	IsAllowed(accessType AccessType, userId string, path guble.Path) bool
}

//AllowAllAccessManager  is a dummy implementation that grants access for everything
type AllowAllAccessManager bool

func NewAllowAllAccessManager(allowAll bool) AllowAllAccessManager {
	return AllowAllAccessManager(allowAll)
}

func (am AllowAllAccessManager) IsAllowed(accessType AccessType, userId string, path guble.Path) bool {
	return bool(am)
}

type RestAccessManager string

func NewRestAccessManager(url string) RestAccessManager {
	return RestAccessManager(url)
}

func (am RestAccessManager) IsAllowed(accessType AccessType, userId string, path guble.Path) bool {

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
		guble.Warn("RestAccessManager: %v", err)
		return false
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	if err != nil || resp.StatusCode != 200 {
		guble.Info("error getting permission", err)
		guble.Debug("error getting permission", responseBody)
		return false
	}

	guble.Debug("RestAccessManager: %v, %v, %v, %v", accessType, userId, path, string(responseBody))

	return "true" == string(responseBody)

}
