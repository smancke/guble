package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_removeTrailingSlash(t *testing.T) {
	cases := []struct {
		expected, path string
	}{
		{"/foo/user/marvin", "/foo/user/marvin"},
		{"/foo/user/marvin", "/foo/user/marvin/"},
		{"/", "/"},
	}

	for i, c := range cases {
		assert.Equal(t, c.expected, removeTrailingSlash(c.path), fmt.Sprintf("Failed at  case no=%d", i))
	}
}
