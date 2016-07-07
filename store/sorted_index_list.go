package store

import (
	"github.com/Sirupsen/logrus"
)

// SortedIndexList a sorted list of fetch entries
type SortedIndexList []*FetchEntry

func (pq *SortedIndexList) Len() int { return len(*pq) }

// InsertList will merge the given list as a parameter in current list
func (pq *SortedIndexList) InsertList(list *SortedIndexList) {
	for _, el := range *list {
		pq.Insert(el)
	}
}

//Insert  adds in the sorted list a new element
func (pq *SortedIndexList) Insert(newElement *FetchEntry) {
	//messageStoreLogger.WithField("new_msgID", newElement.msgID).Info("Adding on list")

	// first element on list just append at the end
	if pq.Len() == 0 {
		*pq = append(*pq, newElement)
		return
	}

	// if the first element in list have a bigger id...insert new element on the start of list
	if (*pq)[0].messageID >= newElement.messageID {
		*pq = append(SortedIndexList{newElement}, *pq...)
		return
		//to optimize the performance check if the new element is having a bigger ID than the last one
	}

	if (*pq)[pq.Len()-1].messageID <= newElement.messageID {
		*pq = append(*pq, newElement)
		return
	}

	//found the correct position to make an insertion sort
	for i := 1; i <= pq.Len()-1; i++ {
		if (*pq)[i].messageID > newElement.messageID {
			pq.insertAt(i, newElement)
			return
		}
	}
}

func (pq *SortedIndexList) insertAt(index int, newElement *FetchEntry) {
	*pq = append((*pq)[:index], append(SortedIndexList{newElement}, (*pq)[index:]...)...)
}

// Clear empties the current list
func (pq *SortedIndexList) Clear() {
	*pq = make(SortedIndexList, 0)
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
func (pq *SortedIndexList) GetIndexEntryFromID(searchID uint64) (bool, int, int, *FetchEntry) {
	if pq.Len() == 0 {
		return false, -1, -1, nil
	}

	h := pq.Len() - 1
	l := 0
	bestIndex := l
	for l <= h {
		mid := (h + l) / 2
		if (*pq)[mid].messageID == searchID {
			return true, mid, bestIndex, (*pq)[mid]
		} else if (*pq)[mid].messageID < searchID {
			l = mid + 1
		} else {
			h = mid - 1
		}

		if abs((*pq)[mid].messageID, searchID) <= abs((*pq)[bestIndex].messageID, searchID) {
			bestIndex = mid
		}
	}

	return false, -1, bestIndex, nil
}

//Back retrieves the element with the biggest id or nil if list is empty
func (pq *SortedIndexList) Back() *FetchEntry {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[pq.Len()-1]
}

//Front retrieves the element with the smallest id or nil if list is empty
func (pq *SortedIndexList) Front() *FetchEntry {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[0]
}

//Front retrieves the element at the given index or nil if position is incorrect or list is empty
func (pq *SortedIndexList) Get(pos int) *FetchEntry {
	if pq.Len() == 0 || pos < 0 || pos >= pq.Len() {
		messageStoreLogger.WithFields(logrus.Fields{
			"len":   pq.Len(),
			"pos":     pos,
		}).Info("Empty list or invalid index")
		return nil
	}
	return (*pq)[pos]
}

//TODO remove after usage.(ONly for testing )
func (pq *SortedIndexList) PrintPq() {
	for i := 0; i < pq.Len(); i++ {
		messageStoreLogger.WithFields(logrus.Fields{
			"msgSize":   (*pq)[i].size,
			"msgId":     (*pq)[i].messageID,
			"msgOffset": (*pq)[i].offset,
		}).Debug("Printing element")
	}
}

func newList(size int) *SortedIndexList {
	pq := make(SortedIndexList, 0, size)
	return &pq
}
