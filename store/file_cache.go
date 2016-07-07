package store

import "sync"

type FileCacheEntry struct {
	minMsgID uint64
	maxMsgID uint64
}

type fileCache struct {
	entries []*FileCacheEntry
	cacheMutex sync.RWMutex
}

func newCache() *fileCache {
	fc := &fileCache{
		entries:     make([]*FileCacheEntry, 0),
	}
	return fc
}

func (fc *fileCache) Len() int {
	fc.cacheMutex.RLock()
	defer  fc.cacheMutex.RUnlock()
	return len(fc.entries)
}

func (fc *fileCache) Add(newEl *FileCacheEntry) {
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()
	fc.entries = append(fc.entries, newEl)
}

func (f *FileCacheEntry) hasStartID(req *FetchRequest) bool {
	if req.StartID == 0 {
		req.Direction = 1
		return true
	}

	if req.Direction >= 0 {
		return req.StartID >= f.minMsgID && req.StartID <= f.maxMsgID
	} else {
		return req.StartID >= f.minMsgID
	}
}