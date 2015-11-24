package guble

// This scruct represents a message in the guble protocol, as the server sends to the client.
type Message struct {

	// The sequenceId of the message, which is given by the
	// server an is strictly monotonically increasing at least within a root topic.
	Id int64

	// The topic path
	Path Path

	// The user id of the message sender
	PublisherUserId string

	// The id of the sending application
	PublisherApplicationId string

	// An id given by the sender (optional)
	PublisherMessageId string

	// The time of publishing, as iso date string
	PublishingTime string

	// The header line of the message (optional). If set, than this has to be a valid json object structure.
	HeaderJson string

	// The message payload
	Body []byte
}

// The path of a topic
type Path string

func ParseMessage(message []byte) (*Message, error) {
	return &Message{
		Body: message,
	}, nil
}
