package file

import (
	"sync"

	"github.com/Sirupsen/logrus"
)

// SortedIndexList a sorted list of fetch entries
type IndexList struct {
	sync.RWMutex
	items []*Index
}

func newList(size int) *IndexList {
	return &IndexList{items: make([]*Index, 0, size)}
}

func (l *IndexList) Len() int { return len(*l) }

//Insert  adds in the sorted list a new element
func (l *IndexList) Insert(items ...*Index) {
	for _, elem := range items {
		// first element on list just append at the end
		if l.Len() == 0 {
			l.items = append(l.items, elem)
			return
		}

		// if the first element in list have a bigger id...insert new element on the start of list
		if l.items[0].messageID >= elem.messageID {
			l.items = append(IndexList{elem}, l.items)
			return
		}

		if l.items[l.Len()-1].messageID <= elem.messageID {
			l.items = append(l.items, elem)
			return
		}

		//found the correct position to make an insertion sort
		for i := 1; i <= l.Len()-1; i++ {
			if l.items[i].messageID > elem.messageID {
				l.insertAt(i, elem)
				return
			}
		}
	}
}

func (l *IndexList) insertAt(index int, newElement *Index) {
	*l = append((*l)[:index], append(IndexList{newElement}, (*l)[index:]...)...)
}

// Clear empties the current list
func (l *IndexList) Clear() {
	*l = make(IndexList, 0)
}

//returns the absolute value for two numbers
func abs(m1, m2 uint64) uint64 {
	if m1 > m2 {
		return m1 - m2
	}

	return m2 - m1
}

// GetIndexEntryFromID performs a binarySearch retrieving the
// true, the position and list and the actual entry if found
// false , -1 ,nil if position is not found
func (l *IndexList) GetIndexEntryFromID(searchID uint64) (bool, int, int, *Index) {
	if l.Len() == 0 {
		return false, -1, -1, nil
	}

	h := l.Len() - 1
	l := 0
	bestIndex := l
	for l <= h {
		mid := (h + l) / 2
		if (*l)[mid].messageID == searchID {
			return true, mid, bestIndex, (*l)[mid]
		} else if (*l)[mid].messageID < searchID {
			l = mid + 1
		} else {
			h = mid - 1
		}

		if abs((*l)[mid].messageID, searchID) <= abs((*l)[bestIndex].messageID, searchID) {
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
	return (*l)[l.Len()-1]
}

//Front retrieves the element with the smallest id or nil if list is empty
func (l *IndexList) Front() *Index {
	if l.Len() == 0 {
		return nil
	}
	return (*l)[0]
}

//Front retrieves the element at the given index or nil if position is incorrect or list is empty
func (l *IndexList) Get(pos int) *Index {
	if l.Len() == 0 || pos < 0 || pos >= l.Len() {
		messageStoreLogger.WithFields(logrus.Fields{
			"len": l.Len(),
			"pos": pos,
		}).Info("Empty list or invalid index")
		return nil
	}
	return (*l)[pos]
}
