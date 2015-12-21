package server

import (
	guble "github.com/smancke/guble/guble"

	"github.com/rs/xid"

	"net/http"
)

type Route struct {
	Id                 string
	Path               guble.Path
	C                  chan *guble.Message
	CloseRouteByRouter chan string
	UserId             string
	ApplicationId      string
}

func NewRoute(path string, channel chan *guble.Message, closeRouteByRouter chan string, applicationId string, userId string) *Route {
	return &Route{
		Id:                 xid.New().String(),
		Path:               guble.Path(path),
		C:                  channel,
		CloseRouteByRouter: closeRouteByRouter,
		UserId:             userId,
		ApplicationId:      applicationId,
	}
}

type PubSubSource interface {
	Subscribe(r *Route) *Route
	Unsubscribe(r *Route)
}

type MessageSink interface {
	HandleMessage(message *guble.Message)
}

// WSConn is a wrapper interface for the needed functions of the websocket.Conn
// It is introduced for testability of the WSHandler
type WSConn interface {
	Close()
	Send(bytes []byte) (err error)
	Receive(bytes *[]byte) (err error)
}

type Startable interface {
	Start()
}

type Stopable interface {
	Stop() error
}

type SetRouter interface {
	SetRouter(router PubSubSource)
}

// interface for modules, which need a MessageEntry set
type SetMessageEntry interface {
	SetMessageEntry(messageSink MessageSink)
}

type Endpoint interface {
	http.Handler
	GetPrefix() string
}

// A fetch request for fetching messages in a MessageStore
type FetchRequest struct {

	// The partition to search for messages
	Partition              string

	// The message sequence id to start
	StartId                uint64

	// A topic path to filter
	TopicPath              guble.Path

	// The maximum number of messages to return
	// AdditionalMessageCount == 0: Only the Message with StartId
	// AdditionalMessageCount >0: Fetch also the next AdditionalMessageCount Messages with a higher MessageId
	// AdditionalMessageCount <0: Fetch also the next AdditionalMessageCount Messages with a lower MessageId
	AdditionalMessageCount int

	// The cannel to send the message back to the receiver
	MessageC               chan *guble.Message

	// A Callback if an error occures
	ErrorCallback          chan error
}

// Interface for a persistance backend storing topics
type MessageStore interface {

	// store a message within a partition
	Store(partition string, msg *guble.Message) error

	// fetch a set of messages
	Fetch(FetchRequest)
}

// Interface for a persistance backend storing key value pairs
type KVStore interface {
	Put(schema, key string, value []byte) error
	Get(schema, key string) (value []byte, exist bool, err error)
	Delete(schema, key string) error
}

// Interface for modules, which need a Key Value store set,
// for storing their data
type SetKVStore interface {
	SetKVStore(kvStore KVStore)
}
