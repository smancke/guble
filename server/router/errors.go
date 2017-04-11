package router

import (
	"errors"
	"fmt"
)

var (
	// ErrServiceNotProvided is returned when the service required is not set.
	ErrServiceNotProvided = errors.New("Service not provided.")

	// ErrInvalidRoute is returned by the `Deliver` method of a `Route` when it has been closed
	// due to slow processing
	ErrInvalidRoute = errors.New("Route is invalid. Channel is closed.")

	// ErrChannelFull is returned when trying to `Deliver` a message with a queue size of zero
	// and the channel is full
	ErrChannelFull = errors.New("Route channel is full. Route is closed.")

	// ErrQueueFull is returned when trying to `Deliver` a message in a full queued route
	ErrQueueFull = errors.New("Route queue is full. Route is closed.")
)

// ModuleStoppingError is returned when the module is stopping
type ModuleStoppingError struct {
	Name string
}

func (m *ModuleStoppingError) Error() string {
	return fmt.Sprintf("Service %s is stopping", m.Name)
}
