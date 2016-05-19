package auth

import (
	"fmt"
	"github.com/smancke/guble/guble"
)

type TestAccessManager struct {
	access map[string]map[guble.Path]bool
}
//should be removed and test rewritten with proper mocking
func NewTestAccessManager() *TestAccessManager {
	return &TestAccessManager{
		access: make(map[string]map[guble.Path]bool),
	}
}

func (tam *TestAccessManager) Allow(userId string, path guble.Path) {
	v, ok := tam.access[userId]
	if !ok {
		v = make(map[guble.Path]bool)
		tam.access[userId] = v
	}
	v[path] = true
}

func (tam *TestAccessManager) IsAllowed(accessType AccessType, userId string, path guble.Path) bool {
	fmt.Print("AccessAllowed: ", userId, path)
	v, ok := tam.access[userId]
	if ok {
		_, ok = v[path]
		fmt.Println(" : true")
		return ok
	}
	fmt.Println(" : false")
	return false
}
