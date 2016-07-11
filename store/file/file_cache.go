package file

import (
	"sync"

	"github.com/smancke/guble/store"
)

type cache struct {
	entries    []*cacheEntry
	cacheMutex sync.RWMutex
}

func newCache() *cache {
	fc := &cache{
		entries: make([]*cacheEntry, 0),
	}
	return fc
}

func (fc *cache) Len() int {
	fc.cacheMutex.RLock()
	defer fc.cacheMutex.RUnlock()
	return len(fc.entries)
}

func (fc *cache) Append(newEl *cacheEntry) {
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()
	fc.entries = append(fc.entries, newEl)
}

type cacheEntry struct {
	min, max uint64
}

func (f *cacheEntry) has(req *store.FetchRequest) bool {
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
