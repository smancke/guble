package server

import (
	"github.com/golang/mock/gomock"
	"testing"
)

var ctrl *gomock.Controller

func initCtrl(t *testing.T) func() {
	ctrl = gomock.NewController(t)
	return func() { ctrl.Finish() }
}
