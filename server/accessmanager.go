package server
import (
	"github.com/smancke/guble/guble"
	"fmt"
	"net/http"
	"io"
	"bytes"
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
	url := string(am)+"?type="
	if (accessType == READ) {
		url += "read"
	} else {
		url += "write"
	}
	url += "&user="+ userId
	url += "&path="+string(path)
	resp, err := http.DefaultClient.Get(url)
	defer resp.Close()

	if(err != nil) {
		//TODO log error
		return false
	}

	buff := make([]byte, 4, 4)
	io.ReadFull(resp.Body, buff)
	if("true" == string(buff)) {
		return true
	}


	return false

}
