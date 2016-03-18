package server
import (
	"testing"
	"github.com/smancke/guble/guble"
	"github.com/stretchr/testify/assert"
	"fmt"
)

type TestAccessManager struct {
	access map[string]map[guble.Path]bool
}

func NewTestAccessManager() *TestAccessManager {
	return &TestAccessManager{
		access:make(map[string]map[guble.Path]bool),
	}
}

func (tam *TestAccessManager ) allow(userId string, path guble.Path) {
	v, ok := tam.access[userId];
	if(!ok) {
		v = make(map[guble.Path]bool);
		tam.access[userId] = v;
	}
	v[path] = true;
}

func (tam *TestAccessManager ) AccessAllowed(accessType AccessType, userId string, path guble.Path) bool {
	fmt.Print("AccessAllowed: ", userId, path)
	v, ok := tam.access[userId]
	if(ok) {
		_ , ok = v[path];
		fmt.Println(" : true")
		return ok
	}
	fmt.Println(" : false")
	return false;
}

func Test_AllowAllAccessManager(t *testing.T) {
	a := assert.New(t)
	am := AccessManager(NewAllowAllAccessManager(true))
	a.True(am.AccessAllowed(READ, "userid", "/path"))

	am = AccessManager(NewAllowAllAccessManager(false))
	a.False(am.AccessAllowed(READ, "userid", "/path"))

}

// this test needs an external service running
// TODO create mock service for testing
func Dont_Test_RestAccessManager(t *testing.T) {
	a := assert.New(t)
	am := NewRestAccessManager("http://localhost:9000/follow/accessAllowed")
	a.False(am.AccessAllowed(READ, "user", "/foo"))
	a.True(am.AccessAllowed(READ, "foo", "/foo"))
	a.True(am.AccessAllowed(WRITE, "foo", "/foo"))

	a.False(am.AccessAllowed(READ, "user", "invalidpath"))
}