package apns

import (
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNew_WithoutKVStore(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	mRouter := NewMockRouter(testutil.MockCtrl)
	errKVS := errors.New("No KVS was set-up in Router")
	mRouter.EXPECT().KVStore().Return(nil, errKVS).AnyTimes()
	mSender := NewMockSender(testutil.MockCtrl)
	prefix := "/apns/"
	workers := 1
	cfg := Config{
		Prefix:  &prefix,
		Workers: &workers,
	}

	//when
	c, err := New(mRouter, mSender, cfg)

	//then
	a.Error(err)
	a.Nil(c)
}

func TestConn_HandleResponseOnSendError(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	c := newAPNSConnector(t)
	mRequest := NewMockRequest(testutil.MockCtrl)
	e := errors.New("A Sender error")

	//when
	err := c.HandleResponse(mRequest, nil, e)

	//then
	a.Equal(e, err)
}

func TestConn_HandleResponse(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	c := newAPNSConnector(t)

	mSubscriber := NewMockSubscriber(testutil.MockCtrl)
	mSubscriber.EXPECT().SetLastID(gomock.Any())

	message := &protocol.Message{
		ID: 42,
	}
	mRequest := NewMockRequest(testutil.MockCtrl)
	mRequest.EXPECT().Message().Return(message).AnyTimes()
	mRequest.EXPECT().Subscriber().Return(mSubscriber).AnyTimes()

	response := &apns2.Response{
		ApnsID:     "id-life",
		StatusCode: 200,
	}

	//when
	err := c.HandleResponse(mRequest, response, nil)

	//then
	a.NoError(err)
}

func TestNew_HandleResponseHandleSubscriber(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	c := newAPNSConnector(t)

	removeForReasons := []string{
		apns2.ReasonMissingDeviceToken,
		apns2.ReasonBadDeviceToken,
		apns2.ReasonDeviceTokenNotForTopic,
		apns2.ReasonUnregistered,
	}
	for _, reason := range removeForReasons {
		message := &protocol.Message{
			ID: 42,
		}
		mSubscriber := NewMockSubscriber(testutil.MockCtrl)
		mSubscriber.EXPECT().SetLastID(gomock.Any())
		mSubscriber.EXPECT().Cancel()
		mSubscriber.EXPECT().Key().Return("key")

		mRequest := NewMockRequest(testutil.MockCtrl)
		mRequest.EXPECT().Message().Return(message).AnyTimes()
		mRequest.EXPECT().Subscriber().Return(mSubscriber).AnyTimes()

		response := &apns2.Response{
			ApnsID:     "id-life",
			StatusCode: 400,
			Reason:     reason,
		}

		//when
		err := c.HandleResponse(mRequest, response, nil)

		//then
		a.NoError(err)
	}
}

func TestNew_HandleResponseDoNotHandleSubscriber(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	c := newAPNSConnector(t)

	noActionForReasons := []string{
		apns2.ReasonPayloadEmpty,
		apns2.ReasonPayloadTooLarge,
		apns2.ReasonBadTopic,
		apns2.ReasonTopicDisallowed,
		apns2.ReasonBadMessageID,
		apns2.ReasonBadExpirationDate,
		apns2.ReasonBadPriority,
		apns2.ReasonDuplicateHeaders,
		apns2.ReasonBadCertificateEnvironment,
		apns2.ReasonBadCertificate,
		apns2.ReasonForbidden,
		apns2.ReasonBadPath,
		apns2.ReasonMethodNotAllowed,
		apns2.ReasonTooManyRequests,
		apns2.ReasonIdleTimeout,
		apns2.ReasonShutdown,
		apns2.ReasonInternalServerError,
		apns2.ReasonServiceUnavailable,
		apns2.ReasonMissingTopic,
	}

	for _, reason := range noActionForReasons {
		message := &protocol.Message{
			ID: 42,
		}
		mSubscriber := NewMockSubscriber(testutil.MockCtrl)
		mSubscriber.EXPECT().SetLastID(gomock.Any())

		mRequest := NewMockRequest(testutil.MockCtrl)
		mRequest.EXPECT().Message().Return(message).AnyTimes()
		mRequest.EXPECT().Subscriber().Return(mSubscriber).AnyTimes()

		response := &apns2.Response{
			ApnsID:     "id-apns",
			StatusCode: 400,
			Reason:     reason,
		}

		//when
		err := c.HandleResponse(mRequest, response, nil)

		//then
		a.NoError(err)
	}
}

func TestConn_Check(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	c := newAPNSConnector(t)
	assert.Nil(t, c.Check())
}

func newAPNSConnector(t *testing.T) connector.ReactiveConnector {
	mRouter := NewMockRouter(testutil.MockCtrl)
	mKVS := NewMockKVStore(testutil.MockCtrl)
	mRouter.EXPECT().KVStore().Return(mKVS, nil).AnyTimes()
	mSender := NewMockSender(testutil.MockCtrl)

	prefix := "/apns/"
	workers := 1
	cfg := Config{
		Prefix:  &prefix,
		Workers: &workers,
	}
	c, err := New(mRouter, mSender, cfg)

	assert.NoError(t, err)
	assert.NotNil(t, c)
	return c
}
