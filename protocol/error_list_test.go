package protocol

import (
	"github.com/stretchr/testify/assert"

	"errors"
	"testing"
)

func TestErrorList(t *testing.T) {
	a := assert.New(t)

	l := NewErrorList("bad things happened: ")
	a.NoError(l.ErrorOrNil())

	l.Add(errors.New("lost in rain"))
	l.Add(errors.New("alone in the dessert"))

	a.Error(l.ErrorOrNil())
	a.Equal(l.ErrorOrNil().Error(), "bad things happened: lost in rain; alone in the dessert")
}
