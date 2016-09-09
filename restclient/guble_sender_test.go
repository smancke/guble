package restclient

import (
	"github.com/stretchr/testify/assert"
	"net/url"
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
		expected string
	}{
		"endpoint only, no topic, no user, no params": {
			endpoint: "http://localhost:8080/api",
			expected: "http://localhost:8080/api/?userId=",
		},
		"endpoint, valid topic, no user, no params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			expected: "http://localhost:8080/api/topic?userId=",
		},
		"endpoint, valid topic, valid user, no params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			expected: "http://localhost:8080/api/topic?userId=user",
		},
		"endpoint, valid topic, valid user, empty params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params:   map[string]string{},
			expected: "http://localhost:8080/api/topic?userId=user",
		},
		"endpoint, valid topic, valid user, one valid param": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params:   map[string]string{"filterCriteria1": "value1"},
			expected: "http://localhost:8080/api/topic?filterCriteria1=value1&userId=user",
		},
		"endpoint, valid topic, valid user, more valid params": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params: map[string]string{
				"filterCriteria1": "value1",
				"filterCriteria2": "value2",
			},
			expected: "http://localhost:8080/api/topic?filterCriteria1=value1&filterCriteria2=value2&userId=user",
		},
		"endpoint, valid topic, valid user, one param value invalid inside URL": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params:   map[string]string{"filterCriteria1": "?"},
			expected: "http://localhost:8080/api/topic?filterCriteria1=%3F&userId=user",
		},
		"endpoint, valid topic, valid user, one param key empty": {
			endpoint: "http://localhost:8080/api",
			topic:    "topic",
			userID:   "user",
			params:   map[string]string{"": "value"},
			expected: "http://localhost:8080/api/topic?userId=user",
		},
	}

	var err error
	for name, c := range testcases {
		_, err = url.Parse(c.expected)
		a.NoError(err)
		a.Equal(c.expected,
			getURL(c.endpoint, c.topic, c.userID, c.params),
			"Failed check for case: "+name)
	}

}
