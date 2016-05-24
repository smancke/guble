package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func Test_PrintHelp(t *testing.T) {
	expectedHelpMessage := `
## Commands
?           # print this info

+ /foo/bar  # subscribe to the topic /foo/bar
+ /foo 0    # read from message 0 and subscribe to the topic /foo
+ /foo 0 5  # read messages 0-5 from /foo
+ /foo -5   # read the last 5 messages and subscribe to the topic /foo

- /foo      # cancel the subscription for /foo

> /foo         # send a message to /foo
> /foo/bar 42  # send a message to /foo/bar with publisherid 42
` + "\n"

	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printHelp()

	w.Close()
	out, _ := ioutil.ReadAll(r)
	os.Stdout = rescueStdout

	resultMessage := fmt.Sprintf("%s", out)
	assert.Equal(t, expectedHelpMessage, resultMessage)

}

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
