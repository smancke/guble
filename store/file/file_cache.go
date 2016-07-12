package file

import (
	"sync"

	"github.com/smancke/guble/store"
)

type cache struct {
	entries []*cacheEntry
	sync.RWMutex
}

func newCache() *cache {
	fc := &cache{
		entries: make([]*cacheEntry, 0),
	}
	return fc
}

func (fc *cache) Len() int {
	fc.RLock()
	defer fc.RUnlock()

	return len(fc.entries)
}

func (fc *cache) Append(entry *cacheEntry) {
	fc.Lock()
	defer fc.Unlock()

	fc.entries = append(fc.entries, entry)
}

type cacheEntry struct {
	min, max uint64
}

// Contains returns true if the req.StartID is between the min and max
// There is a chance the request messages to be found in this range
func (f *cacheEntry) Contains(req *store.FetchRequest) bool {
	if req.StartID == 0 {
		req.Direction = 1
		return true
	}

	if req.Direction >= 0 {
		return req.StartID >= f.min && req.StartID <= f.max
	} else {
		return req.StartID >= f.min
	}
}
