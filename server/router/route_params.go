package router

import (
	"fmt"
	"sort"
	"strings"
)

type RouteParams map[string]string

func (rp *RouteParams) String() string {
	s := make([]string, 0, len(*rp))
	for k, v := range *rp {
		s = append(s, fmt.Sprintf("%s:%s", k, v))
	}
	return strings.Join(s, " ")
}

func (rp *RouteParams) Key() string {
	// The generated key must be the same always
	s := make([]string, 0, len(*rp))
	for _, k := range rp.orderedKeys() {
		s = append(s, fmt.Sprintf("%s:%s", k, (*rp)[k]))
	}
	return strings.Join(s, " ")
}

// orderedKeys returns a slice of ordered
func (rp *RouteParams) orderedKeys() []string {
	keys := make([]string, len(*rp))
	i := 0
	for k := range *rp {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

// Equal verifies if the `receiver` params are the same as `other` params.
// The `keys` param specifies which keys to check in case the match has to be
// done only on a separate set of keys and not on all keys.
func (rp *RouteParams) Equal(other RouteParams, keys ...string) bool {
	if len(keys) > 0 {
		return rp.partialEqual(other, keys)
	}
	if len(*rp) != len(other) {
		return false
	}
	for k, v := range *rp {
		if v2, ok := other[k]; !ok {
			return false
		} else if v != v2 {
			return false
		}
	}
	return true
}

func (rp *RouteParams) partialEqual(other RouteParams, fields []string) bool {
	for _, key := range fields {
		if v, ok := other[key]; !ok {
			return false
		} else if v != (*rp)[key] {
			return false
		}
	}
	return true
}

func (rp *RouteParams) Get(key string) string {
	return (*rp)[key]
}

func (rp *RouteParams) Set(key, value string) {
	(*rp)[key] = value
}
