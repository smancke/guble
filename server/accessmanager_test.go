package server
import (
	"testing"
	"github.com/smancke/guble/guble"
	"github.com/stretchr/testify/assert"
	"fmt"
	"net/http/httptest"
	"net/http"
)

type TestAccessManager struct {
	access map[string]map[guble.Path]bool
}

func NewTestAccessManager() *TestAccessManager {
	return &TestAccessManager{
		access:make(map[string]map[guble.Path]bool),
	}
}

func (tam *TestAccessManager ) allow(userId string, path guble.Path) {
	v, ok := tam.access[userId];
	if(!ok) {
		v = make(map[guble.Path]bool);
		tam.access[userId] = v;
	}
	v[path] = true;
}

func (tam *TestAccessManager ) AccessAllowed(accessType AccessType, userId string, path guble.Path) bool {
	fmt.Print("AccessAllowed: ", userId, path)
	v, ok := tam.access[userId]
	if(ok) {
		_ , ok = v[path];
		fmt.Println(" : true")
		return ok
	}
	fmt.Println(" : false")
	return false;
}

func Test_AllowAllAccessManager(t *testing.T) {
	a := assert.New(t)
	am := AccessManager(NewAllowAllAccessManager(true))
	a.True(am.AccessAllowed(READ, "userid", "/path"))

	am = AccessManager(NewAllowAllAccessManager(false))
	a.False(am.AccessAllowed(READ, "userid", "/path"))

}


func Test_RestAccessManager_Allowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("true"))
	}))

	defer ts.Close()
	a := assert.New(t)
	am := NewRestAccessManager(ts.URL)
	a.True(am.AccessAllowed(READ, "foo", "/foo"))
	a.True(am.AccessAllowed(WRITE, "foo", "/foo"))

}

func Test_RestAccessManager_Not_Allowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("false"))
	}))

	defer ts.Close()
	am := NewRestAccessManager(ts.URL)
	a := assert.New(t)
	a.False(am.AccessAllowed(READ, "user", "/foo"))

}