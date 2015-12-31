package server

import (
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/smancke/guble/guble"
	"time"
)

func Test_MessageEntry_MessagesIsStored_And_GetsCorrectParameters(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	startTime := time.Now()

	msg := &guble.Message{Path: guble.Path("/topic1")}
	var storedMsg []byte
	var routedMsg *guble.Message
	routerMock := NewMockMessageSink(ctrl)
	messageEntry := NewMessageEntry(routerMock)
	messageStoreMock := NewMockMessageStore(ctrl)
	messageEntry.SetMessageStore(messageStoreMock)

	messageStoreMock.EXPECT().MaxMessageId("topic1").Return(uint64(41), nil)
	messageStoreMock.EXPECT().Store("topic1", uint64(42), gomock.Any()).
		Do(func(topic string, id uint64, msg []byte) {
		storedMsg = msg
	})

	routerMock.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *guble.Message) {
		routedMsg = msg
		a.Equal(uint64(42), msg.Id)
		t, e := time.Parse(time.RFC3339, msg.PublishingTime) // publishing time
		a.NoError(e)
		a.True(t.After(startTime.Add(-1 * time.Second)))
		a.True(t.Before(time.Now().Add(time.Second)))
	})

	messageEntry.HandleMessage(msg)

	a.Equal(routedMsg.Bytes(), storedMsg)
}

func Test_MessageEntry_getPartitionFromTopic(t *testing.T) {
	a := assert.New(t)
	messageEntry := &MessageEntry{}
	a.Equal("foo", messageEntry.getPartitionFromTopic(guble.Path("/foo/bar/bazz")))
	a.Equal("foo", messageEntry.getPartitionFromTopic(guble.Path("/foo")))
	a.Equal("", messageEntry.getPartitionFromTopic(guble.Path("/")))
	a.Equal("", messageEntry.getPartitionFromTopic(guble.Path("")))
}
