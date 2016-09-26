package restclient

// Sender is an interface used to send a message to the guble server.
type Sender interface {
	// Send a a message(body) to the guble Server, to the given topic, with the given userID.
	Send(topic string, body []byte, userID string, params map[string]string) error

	// Check returns `true` if the guble server endpoint is reachable, or `false` otherwise.
	Check() bool

	// GetSubscribers returns a binary encoded JSON of all subscribers of 'topic' or an error otherwise
	GetSubscribers(topic string) ([]byte, error)
}
