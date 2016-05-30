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

// ServiceStoppingError is returned when a service is in stopping process
type ServiceStoppingError struct {
	Name string
}

func (e *ServiceStoppingError) Error() string {
	return fmt.Sprintf("Service %s stopping", e.Name)
}
