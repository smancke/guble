package server
import (
	"github.com/smancke/guble/guble"
	"fmt"
	"net/http"
	"io/ioutil"
)


type AllowAllAccessManager bool

func NewAllowAllAccessManager(allowAll bool) AllowAllAccessManager {
	return AllowAllAccessManager(allowAll)
}

func (am AllowAllAccessManager) AccessAllowed(accessType AccessType, userId string, path guble.Path) bool {
	return bool(am);
}

type RestAccessManager string

func NewRestAccessManager(url string) RestAccessManager {
	return RestAccessManager(url)
}

func (am RestAccessManager) AccessAllowed(accessType AccessType, userId string, path guble.Path) bool {
	url := string(am) + "?type="
	if (accessType == READ) {
		url += "read"
	} else {
		url += "write"
	}

	url += "&userId=" + userId
	url += "&path=" + string(path)

	resp, err := http.DefaultClient.Get(url)

	if (err != nil) {
		guble.Warn("RestAccessManager: %v", err)
		return false
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	if (err != nil || resp.StatusCode != 200) {
		fmt.Println("RestAccessManager: ", string(responseBody), err)
		return false
	}

	guble.Debug("RestAccessManager: %v, %v, %v, %v", accessType, userId, path, string(responseBody))

	return "true" == string(responseBody)

}
