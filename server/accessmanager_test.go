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