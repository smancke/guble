package apns

import (
	"errors"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/golang/mock/gomock"
	"github.com/sideshow/apns2"
	"github.com/stretchr/testify/assert"
	"testing"
)

var ErrSendRandomError = errors.New("A Sender error")

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
	c, _ := newAPNSConnector(t)
	mRequest := NewMockRequest(testutil.MockCtrl)

	//when
	err := c.HandleResponse(mRequest, nil, nil, ErrSendRandomError)

	//then
	a.Equal(ErrSendRandomError, err)
}

func TestConn_HandleResponse(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	c, mKVS := newAPNSConnector(t)

	mSubscriber := NewMockSubscriber(testutil.MockCtrl)
	mSubscriber.EXPECT().SetLastID(gomock.Any())
	mSubscriber.EXPECT().Key().Return("key").AnyTimes()
	mSubscriber.EXPECT().Encode().Return([]byte("{}"), nil).AnyTimes()
	mKVS.EXPECT().Put(schema, "key", []byte("{}")).Times(2)

	c.Manager().Add(mSubscriber)

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
	err := c.HandleResponse(mRequest, response, nil, nil)

	//then
	a.NoError(err)
}

func TestNew_HandleResponseHandleSubscriber(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	c, mKVS := newAPNSConnector(t)

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
		mSubscriber.EXPECT().Key().Return("key").AnyTimes()
		mSubscriber.EXPECT().Encode().Return([]byte("{}"), nil).AnyTimes()
		mKVS.EXPECT().Put(schema, "key", []byte("{}")).Times(2)
		mKVS.EXPECT().Delete(schema, "key")

		c.Manager().Add(mSubscriber)

		mRequest := NewMockRequest(testutil.MockCtrl)
		mRequest.EXPECT().Message().Return(message).AnyTimes()
		mRequest.EXPECT().Subscriber().Return(mSubscriber).AnyTimes()

		response := &apns2.Response{
			ApnsID:     "id-life",
			StatusCode: 400,
			Reason:     reason,
		}

		//when
		err := c.HandleResponse(mRequest, response, nil, nil)

		//then
		a.NoError(err)
	}
}

func TestNew_HandleResponseDoNotHandleSubscriber(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	//given
	c, mKVS := newAPNSConnector(t)

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
		mSubscriber.EXPECT().Key().Return("key").AnyTimes()
		mSubscriber.EXPECT().Encode().Return([]byte("{}"), nil).AnyTimes()
		mSubscriber.EXPECT().Cancel()
		mKVS.EXPECT().Put(schema, "key", []byte("{}")).Times(2)
		mKVS.EXPECT().Delete(schema, "key")

		c.Manager().Add(mSubscriber)

		mRequest := NewMockRequest(testutil.MockCtrl)
		mRequest.EXPECT().Message().Return(message).AnyTimes()
		mRequest.EXPECT().Subscriber().Return(mSubscriber).AnyTimes()

		response := &apns2.Response{
			ApnsID:     "id-apns",
			StatusCode: 400,
			Reason:     reason,
		}

		//when
		err := c.HandleResponse(mRequest, response, nil, nil)

		//then
		a.NoError(err)

		c.Manager().Remove(mSubscriber)
	}
}

func newAPNSConnector(t *testing.T) (c connector.ResponsiveConnector, mKVS *MockKVStore) {
	mKVS = NewMockKVStore(testutil.MockCtrl)
	mRouter := NewMockRouter(testutil.MockCtrl)
	mRouter.EXPECT().KVStore().Return(mKVS, nil).AnyTimes()
	mSender := NewMockSender(testutil.MockCtrl)

	prefix := "/apns/"
	workers := 1
	intervalMetrics := false
	password := "test"
	bytes := []byte("test")
	cfg := Config{
		Prefix:              &prefix,
		Workers:             &workers,
		IntervalMetrics:     &intervalMetrics,
		CertificatePassword: &password,
		CertificateBytes:    &bytes,
	}
	c, err := New(mRouter, mSender, cfg)

	assert.NoError(t, err)
	assert.NotNil(t, c)
	return
}
