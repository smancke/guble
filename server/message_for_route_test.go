package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMessageForRouteString(t *testing.T) {
	var mfr = &MessageForRoute{}
	assert.Equal(t, mfr.String(), "Message <nil> for route <nil>")

	mfr.Route = &Route{}
	assert.Equal(t, mfr.String(), "Message <nil> for route ::")

	mfr.Message = &protocol.Message{}
	assert.Equal(t, mfr.String(), "Message 0 for route ::")
}
