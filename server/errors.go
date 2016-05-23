package server

import (
	"errors"
	"fmt"
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server/auth"
)

var (
	// ErrServiceNotProvided is returned when the service required is not set.
	ErrServiceNotProvided = errors.New("Service not provided.")
)

// PermissionDeniedError is returned when AccessManager denies a user request for a topic
type PermissionDeniedError struct {
	// userId of request
	userID string
	// accessType  requested(READ/WRITE)
	acccesType auth.AccessType
	// requested topic
	path guble.Path
}

func (e *PermissionDeniedError) Error() string {
	return fmt.Sprintf("Access Denied for user=[%s] on path=[%s] for Operation=[%s]", e.userID, e.path, e.acccesType)
}
