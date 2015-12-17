package server

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestStartAndStopWSServer(t *testing.T) {

	// given: a configured echo webserver
	server := NewWebServer("localhost:3333")
	server.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := ioutil.ReadAll(r.Body)
		w.Write(bytes)
	})

	// when: I start the server
	server.Start()
	time.Sleep(time.Millisecond * 10)
	addr := server.GetAddr()

	// and: send a testmessage
	resp, err := http.Post("http://"+addr, "text/plain", bytes.NewBufferString("hello"))

	// then: the message is returned
	assert.NoError(t, err)
	responseBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "hello", string(responseBody))

	// and when: we stop the service
	server.Stop()
	time.Sleep(time.Millisecond * 100)

	// then: the next call returns an error
	//       because the server is closed
	c2 := &http.Client{}
	c2.Transport = &http.Transport{DisableKeepAlives: true}
	_, err = c2.Post("http://"+addr, "text/plain", bytes.NewBufferString("hello"))
	assert.Error(t, err)
}

func TestExtractUserId(t *testing.T) {
	assert.Equal(t, "marvin", extractUserId("/foo/user/marvin"))
	assert.Equal(t, "marvin", extractUserId("/user/marvin"))
	assert.Equal(t, "", extractUserId("/"))
}
