package restclient

type Sender interface {
	Send(topic string, body []byte) error
	Check() bool
}
