package auth

import (
	"github.com/cosminrentea/gobbler/protocol"
)

//AllowAllAccessManager is a dummy implementation that grants access for everything.
type AllowAllAccessManager bool

//NewAllowAllAccessManager returns a new AllowAllAccessManager (depending on the passed parameter, always true or always false)
func NewAllowAllAccessManager(allowAll bool) AllowAllAccessManager {
	return AllowAllAccessManager(allowAll)
}

//IsAllowed returns always the same value, given at construction time (true or false).
func (am AllowAllAccessManager) IsAllowed(accessType AccessType, userID string, path protocol.Path) bool {
	return bool(am)
}
