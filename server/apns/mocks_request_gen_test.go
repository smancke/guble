// Automatically generated by MockGen. DO NOT EDIT!
// Source: github.com/smancke/guble/server/connector (interfaces: Request)

package apns

import (
	gomock "github.com/golang/mock/gomock"
	protocol "github.com/smancke/guble/protocol"
	connector "github.com/smancke/guble/server/connector"
)

// Mock of Request interface
type MockRequest struct {
	ctrl     *gomock.Controller
	recorder *_MockRequestRecorder
}

// Recorder for MockRequest (not exported)
type _MockRequestRecorder struct {
	mock *MockRequest
}

func NewMockRequest(ctrl *gomock.Controller) *MockRequest {
	mock := &MockRequest{ctrl: ctrl}
	mock.recorder = &_MockRequestRecorder{mock}
	return mock
}

func (_m *MockRequest) EXPECT() *_MockRequestRecorder {
	return _m.recorder
}

func (_m *MockRequest) Message() *protocol.Message {
	ret := _m.ctrl.Call(_m, "Message")
	ret0, _ := ret[0].(*protocol.Message)
	return ret0
}

func (_mr *_MockRequestRecorder) Message() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Message")
}

func (_m *MockRequest) Subscriber() connector.Subscriber {
	ret := _m.ctrl.Call(_m, "Subscriber")
	ret0, _ := ret[0].(connector.Subscriber)
	return ret0
}

func (_mr *_MockRequestRecorder) Subscriber() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Subscriber")
}
