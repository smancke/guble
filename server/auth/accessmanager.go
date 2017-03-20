package auth

import (
	"github.com/cosminrentea/gobbler/protocol"
)

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
	IsAllowed(accessType AccessType, userID string, path protocol.Path) bool
}
