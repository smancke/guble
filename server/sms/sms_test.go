package gateway

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/smancke/guble/testutil"
)

func TestOptivoSender_Send(t *testing.T) {
	defer testutil.EnableDebugForMethod() ()
	a:=assert.New(t)
	err := SendSms()
	a.NoError(err)

}