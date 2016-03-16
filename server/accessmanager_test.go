package server
import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func Test_AllowAllAccessManager (t *testing.T) {
	a := assert.New(t)
	am := AccessManager(NewAllowAllAccessManager(true))
	a.True(am.AccessAllowed(READ, "userid", "/path"))

	am = AccessManager(NewAllowAllAccessManager(false))
	a.False(am.AccessAllowed(READ, "userid", "/path"))

}

// this test needs an external service running
// TODO create mock service for testing
func Dont_Test_RestAccessManager (t *testing.T) {
	a := assert.New(t)
	am := NewRestAccessManager("http://localhost:9000/follow/accessAllowed")
	a.False(am.AccessAllowed(READ, "user", "/foo"))
	a.True(am.AccessAllowed(READ, "foo", "/foo"))
	a.True(am.AccessAllowed(WRITE, "foo", "/foo"))

	a.False(am.AccessAllowed(READ, "user", "invalidpath"))
}