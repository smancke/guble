package restclient

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetURL(t *testing.T) {
	a := assert.New(t)

	testcases := map[string]struct {
		endpoint string
		topic    string
		userID   string
		params   map[string]string

		// expected result
		result string
	}{
		"endpoint only, no topic, no user, no params": {
			endpoint: "http://localhost:8080/api",
			result:   "http://localhost:8080/api/?userId=",
		},
		"endpoint, valid topic, no user, no params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			result:   "http://localhost:8080/api/topic?userId=",
		},
		"endpoint, valid topic, valid user, no params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			result:   "http://localhost:8080/api/topic?userId=user",
		},
		"endpoint, valid topic, valid user, empty params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params:   map[string]string{},
			result:   "http://localhost:8080/api/topic?userId=user",
		},
		"endpoint, valid topic, valid user, one valid param": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params:   map[string]string{"filterCriteria1": "value1"},
			result:   "http://localhost:8080/api/topic?userId=user&filterCriteria1=value1",
		},
		"endpoint, valid topic, valid user, more valid params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params: map[string]string{
				"filterCriteria1": "value1",
				"filterCriteria2": "value2",
			},
			result: "http://localhost:8080/api/topic?userId=user&filterCriteria1=value1&filterCriteria2=value2",
		},
		"endpoint, valid topic, valid user, one invalid param": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params:   map[string]string{"filterCriteria1": "?"},
			result:   "http://localhost:8080/api/topic?userId=user&filterCriteria1=?",
		},
	}

	for name, c := range testcases {
		a.Equal(c.result,
			getURL(c.endpoint, c.topic, c.userID, c.params),
			"Failed check for case: "+name)
	}

}
