package filestore

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/cosminrentea/gobbler/server/store"
)

// IndexList a sorted list of fetch entries
type indexList struct {
	items []*index

	sync.RWMutex
}

func newIndexList(size int) *indexList {
	return &indexList{items: make([]*index, 0, size)}
}

func (l *indexList) len() int {
	l.RLock()
	defer l.RUnlock()

	return len(l.items)
}

func (l *indexList) insertList(other *indexList) {
	l.insert(other.toSliceArray()...)
}

//Insert  adds in the sorted list a new element
func (l *indexList) insert(items ...*index) {
	for _, elem := range items {
		l.insertElem(elem)
	}
}

func (l *indexList) insertElem(elem *index) {
	l.Lock()
	defer l.Unlock()

	// first element on list just append at the end
	if len(l.items) == 0 {
		l.items = append(l.items, elem)
		return
	}

	// if the first element in list have a bigger id...insert new element on the start of list
	if l.items[0].id >= elem.id {
		l.items = append([]*index{elem}, l.items...)
		return
	}

	if l.items[len(l.items)-1].id <= elem.id {
		l.items = append(l.items, elem)
		return
	}

	//found the correct position to make an insertion sort
	for i := 1; i <= len(l.items)-1; i++ {
		if l.items[i].id > elem.id {
			l.items = append(l.items[:i], append([]*index{elem}, l.items[i:]...)...)
			return
		}
	}
}

// Clear empties the current list
func (l *indexList) clear() {
	l.items = make([]*index, 0)
}

// GetIndexEntryFromID performs a binarySearch retrieving the
// true, the position and list and the actual entry if found
// false , -1 ,nil if position is not found
// search performs a binary search returning:
// - `true` in case the item was found
// - `position` position of the item
// - `bestIndex` the closest index to the searched item if not found.
// - `index` the index if found
func (l *indexList) search(searchID uint64) (bool, int, int, *index) {
	l.RLock()
	defer l.RUnlock()

	if len(l.items) == 0 {
		return false, -1, -1, nil
	}

	h := len(l.items) - 1
	f := 0
	bestIndex := f

	for f <= h {
		mid := (h + f) / 2
		if l.items[mid].id == searchID {
			return true, mid, bestIndex, l.items[mid]
		} else if l.items[mid].id < searchID {
			f = mid + 1
		} else {
			h = mid - 1
		}

		if abs(l.items[mid].id, searchID) <= abs(l.items[bestIndex].id, searchID) {
			bestIndex = mid
		}
	}

	return false, -1, bestIndex, nil
}

//Back retrieves the element with the biggest id or nil if list is empty
func (l *indexList) back() *index {
	l.RLock()
	defer l.RUnlock()

	if len(l.items) == 0 {
		return nil
	}

	return l.items[len(l.items)-1]
}

//Front retrieves the element with the smallest id or nil if list is empty
func (l *indexList) front() *index {
	l.RLock()
	defer l.RUnlock()

	if len(l.items) == 0 {
		return nil
	}

	return l.items[0]
}

func (l *indexList) toSliceArray() []*index {
	l.RLock()
	defer l.RUnlock()

	return l.items
}

//Front retrieves the element at the given index or nil if position is incorrect or list is empty
func (l *indexList) get(pos int) *index {
	l.RLock()
	defer l.RUnlock()

	if len(l.items) == 0 || pos < 0 || pos >= len(l.items) {
		logger.WithFields(log.Fields{
			"len": len(l.items),
			"pos": pos,
		}).Info("Empty list or invalid index")
		return nil
	}

	return l.items[pos]
}

func (l *indexList) mapWithPredicate(predicate func(elem *index, i int) error) error {
	l.RLock()
	defer l.RUnlock()

	for i, elem := range l.items {
		if err := predicate(elem, i); err != nil {
			return err
		}
	}

	return nil
}

func (l *indexList) String() string {
	l.RLock()
	defer l.RUnlock()

	s := ""
	for i, elem := range l.items {
		s += fmt.Sprintf("[%d:%d %d] ", i, elem.id, elem.fileID)
	}
	return s
}

// Contains returns true if given ID is between first and last item in the list
func (l *indexList) contains(id uint64) bool {
	l.RLock()
	defer l.RUnlock()

	if len(l.items) == 0 {
		return false
	}

	if id == 0 {
		return true
	}

	return l.items[0].id <= id && id <= l.items[len(l.items)-1].id
}

// Extract will return a new list containing items requested by the FetchRequest from this list
func (l *indexList) extract(req *store.FetchRequest) *indexList {
	potentialEntries := newIndexList(0)
	found, pos, lastPos, _ := l.search(req.StartID)
	currentPos := lastPos
	if found {
		currentPos = pos
	}

	for potentialEntries.len() < req.Count && currentPos >= 0 && currentPos < l.len() {
		elem := l.get(currentPos)
		logger.WithFields(log.Fields{
			"elem":       *elem,
			"currentPos": currentPos,
			"req":        *req,
		}).Debug("Elem in retrieve")

		if elem == nil {
			logger.WithFields(log.Fields{
				"pos":     currentPos,
				"l.Len":   l.len(),
				"len":     potentialEntries.len(),
				"startID": req.StartID,
				"count":   req.Count,
			}).Error("Error in retrieving from list.Got nil entry")
			break
		}

		potentialEntries.insert(elem)
		currentPos += int(req.Direction)

		// // if we reach req.EndID than we break
		if req.EndID > 0 && elem.id >= req.EndID {
			break
		}
	}
	return potentialEntries
}

func abs(m1, m2 uint64) uint64 {
	if m1 > m2 {
		return m1 - m2
	}

	return m2 - m1
}
