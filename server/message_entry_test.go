package server

import (
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"
	"time"
)

func TestMessagesGetAPublishingTime(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	startTime := time.Now()

	routerMock := NewMockMessageSink(ctrl)
	messageEntry := NewMessageEntry(routerMock)
	messageEntry.SetKVStore(store.NewMemoryKVStore())

	routerMock.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *guble.Message) {
		t, e := time.Parse(time.RFC3339, msg.PublishingTime)
		a.NoError(e)
		a.True(t.After(startTime.Add(-1 * time.Second)))
		a.True(t.Before(time.Now().Add(time.Second)))
	})

	messageEntry.HandleMessage(
		&guble.Message{Path: guble.Path("/topic1")},
	)
}

func TestNextIdForTopic(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	messageEntry := NewMessageEntry(NewMockMessageSink(ctrl))
	messageEntry.SetKVStore(store.NewMemoryKVStore())
	a.Equal(uint64(1), messageEntry.nextIdForTopic("/bli/bla"))
	a.Equal(uint64(2), messageEntry.nextIdForTopic("/bli/bla"))
	a.Equal(uint64(3), messageEntry.nextIdForTopic("/bli/BLUBB"))
	a.Equal(uint64(4), messageEntry.nextIdForTopic("/bli/bla"))

	a.Equal(uint64(1), messageEntry.nextIdForTopic("/another/topic1"))
	a.Equal(uint64(1), messageEntry.nextIdForTopic("/WithoutSubtopic"))
	a.Equal(uint64(1), messageEntry.nextIdForTopic("WithoutLeadingSlash"))
	a.Equal(uint64(1), messageEntry.nextIdForTopic("")) // robus against ""
}

func TestInrementingTheMessageId(t *testing.T) {
	defer initCtrl(t)()

	routerMock := NewMockMessageSink(ctrl)
	messageEntry := NewMessageEntry(routerMock)
	messageEntry.SetKVStore(store.NewMemoryKVStore())

	routerMock.EXPECT().HandleMessage(&messageMatcher{1, "/topic1", "topic1Message1", ""})
	messageEntry.HandleMessage(
		&guble.Message{Path: guble.Path("/topic1"), Body: []byte("topic1Message1")},
	)

	routerMock.EXPECT().HandleMessage(&messageMatcher{2, "/topic1", "topic1Message2", ""})
	messageEntry.HandleMessage(
		&guble.Message{Path: guble.Path("/topic1"), Body: []byte("topic1Message2")},
	)

	routerMock.EXPECT().HandleMessage(&messageMatcher{1, "/topic2", "topic2Message1", ""})
	messageEntry.HandleMessage(
		&guble.Message{Path: guble.Path("/topic2"), Body: []byte("topic2Message1")},
	)
}
