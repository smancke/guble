package server

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestHeadersToJson(t *testing.T) {
	a := assert.New(t)

	a.Equal(`{}`, headersToJson(http.Header{}))

	a.Equal(`{"a":"b","x":"y"}`, headersToJson(http.Header{
		X_HEADER_PREFIX + "a": []string{"b"},
		"foo": []string{"b"},
		X_HEADER_PREFIX + "x": []string{"y"},
	}))
}
