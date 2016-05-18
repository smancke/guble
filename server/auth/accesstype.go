package auth

import "github.com/smancke/guble/guble"

// AccessType permission required by the user
type AccessType int

const (
	// READ permission
	READ AccessType = iota

	// WRITE permission
	WRITE
)

// AccessManager interface allows to provide a custom authentication mechanism
type AccessManager interface {
	AccessAllowed(accessType AccessType, userId string, path guble.Path) bool
}
