package server
import (
	"github.com/smancke/guble/guble"
	"fmt"
	"net/http"
	"io/ioutil"
)


type AllowAllAccessManager bool

func NewAllowAllAccessManager(allowAll bool) AllowAllAccessManager {
	fmt.Println("LOLZ")
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

	defer resp.Body.Close()

	if (err != nil) {

		fmt.Println("FAILZ: %s", err)
		return false
	}

	responseBody, err := ioutil.ReadAll(resp.Body)

	if (err != nil || resp.StatusCode != 200) {
		fmt.Println("FAILZ: %s %s", string(responseBody), err)
		return false
	}

	if ("true" == string(responseBody)) {
		return true
	}


	return false

}
