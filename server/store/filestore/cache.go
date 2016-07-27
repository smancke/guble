package filestore

import (
	"sync"

	"github.com/smancke/guble/server/store"
)

type cache struct {
	entries []*cacheEntry
	sync.RWMutex
}

func newCache() *cache {
	c := &cache{
		entries: make([]*cacheEntry, 0),
	}
	return c
}

func (c *cache) len() int {
	c.RLock()
	defer c.RUnlock()

	return len(c.entries)
}

func (c *cache) add(entry *cacheEntry) {
	c.Lock()
	defer c.Unlock()

	c.entries = append(c.entries, entry)
}

type cacheEntry struct {
	min, max uint64
}

// Contains returns true if the req.StartID is between the min and max
// There is a chance the request messages to be found in this range
func (entry *cacheEntry) Contains(req *store.FetchRequest) bool {
	if req.StartID == 0 {
		req.Direction = 1
		return true
	}

	if req.Direction >= 0 {
		return req.StartID >= entry.min && req.StartID <= entry.max
	} else {
		return req.StartID >= entry.min
	}
}
