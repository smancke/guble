package auth

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
