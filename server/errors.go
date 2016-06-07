package server

import (
	"errors"
	"fmt"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/auth"
)

var (
	// ErrServiceNotProvided is returned when the service required is not set.
	ErrServiceNotProvided = errors.New("Service not provided.")

	// ErrInvalidRoute is returned by the `Deliver` method of a `Route` when it has been closed
	// due to slow processing
	ErrInvalidRoute = errors.New("Route is invalid. Channel is closed.")
)

// PermissionDeniedError is returned when AccessManager denies a user request for a topic
type PermissionDeniedError struct {

	// userId of request
	UserID string

	// accessType  requested(READ/WRITE)
	AccessType auth.AccessType

	// requested topic
	Path protocol.Path
}

func (e *PermissionDeniedError) Error() string {
	return fmt.Sprintf("Access Denied for user=[%s] on path=[%s] for Operation=[%s]", e.UserID, e.Path, e.AccessType)
}

// ModuleStoppingError is returned when the module is stopping
type ModuleStoppingError struct {
	Name string
}

func (m *ModuleStoppingError) Error() string {
	return fmt.Sprintf("Service %s is stopping", m.Name)
}
