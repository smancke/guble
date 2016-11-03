package apns

import (
	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSender_Send(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// given
	routeParams := make(map[string]string)
	routeParams["device_id"] = "1234"
	routeConfig := router.RouteConfig{
		Path:        protocol.Path("path"),
		RouteParams: routeParams,
	}
	route := router.NewRoute(routeConfig)

	mSubscriber := NewMockSubscriber(testutil.MockCtrl)
	mSubscriber.EXPECT().Route().Return(route)

	mRequest := NewMockRequest(testutil.MockCtrl)
	mRequest.EXPECT().Subscriber().Return(mSubscriber)

	mPusher := NewMockPusher(testutil.MockCtrl)
	mPusher.EXPECT().Push(gomock.Any()).Return(nil, nil)

	appTopic := "com.myapp"
	cfg := Config{
		AppTopic: &appTopic,
	}

	// and
	s, err := newSender(mPusher, cfg)
	a.NoError(err)

	// when
	rsp, err := s.Send(mRequest)

	// then
	a.NoError(err)
	a.Nil(rsp)
}
