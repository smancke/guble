package apns

import (
	"github.com/sideshow/apns2"
)

type Pusher interface {
	Push(*apns2.Notification) (*apns2.Response, error)
}

type PushCloser interface {
	Pusher
	Close()
}
