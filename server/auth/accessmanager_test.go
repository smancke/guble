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
	a.True(am.IsAllowed(READ, "userid", "/path"))

	am = AccessManager(NewAllowAllAccessManager(false))
	a.False(am.IsAllowed(READ, "userid", "/path"))

}

func Test_RestAccessManagerAllowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("true"))
	}))

	defer ts.Close()
	a := assert.New(t)
	am := NewRestAccessManager(ts.URL)
	a.True(am.IsAllowed(READ, "foo", "/foo"))
	a.True(am.IsAllowed(WRITE, "foo", "/foo"))

}

func Test_RestAccessManagerNotAllowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("false"))
	}))

	defer ts.Close()
	am := NewRestAccessManager(ts.URL)
	a := assert.New(t)
	a.False(am.IsAllowed(READ, "user", "/foo"))
}

func Test_RestAccessManagerNotAllowedWithServerUnstarted(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("false"))
	}))

	defer ts.Close()
	am := NewRestAccessManager(ts.URL)
	a := assert.New(t)
	a.False(am.IsAllowed(READ, "user", "/foo"))
}

func Test_RestAccessManagerNotAllowedHttpReturningStatusForbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))

	defer ts.Close()
	a := assert.New(t)
	am := NewRestAccessManager(ts.URL)
	a.False(am.IsAllowed(READ, "foo", "/foo"))
	a.False(am.IsAllowed(WRITE, "foo", "/foo"))
}
