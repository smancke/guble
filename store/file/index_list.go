package file

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
)

// SortedIndexList a sorted list of fetch entries
type IndexList struct {
	items []*Index

	sync.RWMutex
}

func newList(size int) *IndexList {
	return &IndexList{items: make([]*Index, 0, size)}
}

func (l *IndexList) Len() int {
	l.RLock()
	defer l.RUnlock()

	return len(l.items)
}

func (l *IndexList) InsertList(other *IndexList) {
	l.Insert(other.Items()...)
}

//Insert  adds in the sorted list a new element
func (l *IndexList) Insert(items ...*Index) {
	for _, elem := range items {
		l.insertElem(elem)
	}
}

func (l *IndexList) insertElem(elem *Index) {
	length := l.Len()

	l.Lock()
	defer l.Unlock()

	// first element on list just append at the end
	if length == 0 {
		l.items = append(l.items, elem)
		return
	}

	// if the first element in list have a bigger id...insert new element on the start of list
	if l.items[0].id >= elem.id {
		l.items = append([]*Index{elem}, l.items...)
		return
	}

	if l.items[length-1].id <= elem.id {
		l.items = append(l.items, elem)
		return
	}

	//found the correct position to make an insertion sort
	for i := 1; i <= length-1; i++ {
		if l.items[i].id > elem.id {
			l.items = append(l.items[:i], append([]*Index{elem}, l.items[i:]...)...)
			return
		}
	}
}

// Clear empties the current list
func (l *IndexList) Clear() {
	l.Lock()
	defer l.Unlock()

	l.items = make([]*Index, 0)
}

// GetIndexEntryFromID performs a binarySearch retrieving the
// true, the position and list and the actual entry if found
// false , -1 ,nil if position is not found
func (l *IndexList) Search(searchID uint64) (bool, int, int, *Index) {
	l.RLock()
	defer l.RUnlock()

	if l.Len() == 0 {
		return false, -1, -1, nil
	}

	h := l.Len() - 1
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
func (l *IndexList) Back() *Index {
	if l.Len() == 0 {
		return nil
	}

	l.RLock()
	defer l.RUnlock()

	return l.items[l.Len()-1]
}

//Front retrieves the element with the smallest id or nil if list is empty
func (l *IndexList) Front() *Index {
	l.RLock()
	defer l.RUnlock()

	if l.Len() == 0 {
		return nil
	}

	return l.items[0]
}

func (l *IndexList) Items() []*Index {
	l.RLock()
	defer l.RUnlock()

	return l.items
}

//Front retrieves the element at the given index or nil if position is incorrect or list is empty
func (l *IndexList) Get(pos int) *Index {
	l.RLock()
	defer l.RUnlock()

	if l.Len() == 0 || pos < 0 || pos >= l.Len() {
		logger.WithFields(log.Fields{
			"len": l.Len(),
			"pos": pos,
		}).Info("Empty list or invalid index")
		return nil
	}

	return l.items[pos]
}

func (l *IndexList) Do(predicate func(elem *Index, i int) error) error {
	l.RLock()
	defer l.RUnlock()

	for i, elem := range l.items {
		if err := predicate(elem, i); err != nil {
			return err
		}
	}

	return nil
}

func (l *IndexList) String() string {
	l.RLock()
	defer l.RUnlock()

	s := ""
	for i, elem := range l.items {
		s += fmt.Sprintf("[%d:%d %d] ", i, elem.id, elem.fileID)
	}
	return s
}

// Contains returns true if given ID is between first and last item in the list
func (l *IndexList) Contains(id uint64) bool {
	if l.Len() == 0 {
		return false
	}
	return l.Front().id <= id && id <= l.Back().id
}

//returns the absolute value for two numbers
func abs(m1, m2 uint64) uint64 {
	if m1 > m2 {
		return m1 - m2
	}

	return m2 - m1
}
