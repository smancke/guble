package store

import (
	"github.com/Sirupsen/logrus"
)

// An Item is something we manage in a priority queue.
type IndexFileEntry struct {
	msgSize uint32 // The value of the item; arbitrary.
	msgID   uint64 // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.

	filename string

	messageOffset uint64
}

// A PriorityQueue implements heap.Interface and holds Items.
type SortedIndexList []*IndexFileEntry

func (pq *SortedIndexList) Len() int { return len(*pq) }

//Insert  adds in the sorted list a new element
func (pq *SortedIndexList) Insert(newElement *IndexFileEntry) {
	//messageStoreLogger.WithField("new_msgID", newElement.msgID).Info("Adding on list")

	// first element on list just append at the end
	if pq.Len() == 0 {
		*pq = append(*pq, newElement)
		return
	} else {
		// if the first element in list have a bigger id...insert new element on the start of list
		if (*pq)[0].msgID >= newElement.msgID {
			*pq = append(SortedIndexList{newElement}, *pq...)
			return
			//to optimize the performance check if the new element is having a bigger ID than the last one
		}   else if (*pq)[pq.Len()-1].msgID <= newElement.msgID {
			*pq = append(*pq, newElement)
			return
		}else {
			//found the correct position to make an insertion sort
			for i := 1; i <= pq.Len()-1; i++ {
				if (*pq)[i].msgID > newElement.msgID {
					pq.insertAt(i, newElement)
					return
				}
			}
		}
	}

}

func (pq *SortedIndexList) insertAt(index int, newElement *IndexFileEntry) {
	*pq = append((*pq)[:index], append(SortedIndexList{newElement}, (*pq)[index:]...)...)
}

// Clear empties the current list
func (pq *SortedIndexList) Clear() {
	*pq = nil
	*pq = make(SortedIndexList, 0)

}

// GetIndexEntryFromID performs a binarySearch retrieving the
// true, the position and list and the actual entry if found
// false , -1 ,nil if position is not found
func (pq *SortedIndexList) GetIndexEntryFromID(searchedId uint64) (bool, int ,*IndexFileEntry) {
	//messageStoreLogger.WithField("searchedId", searchedId).Info("GetIndexEntryFromID")

	if(pq.Len() == 0) {
		return false,-1,nil
	}
	h := pq.Len()-1
	l := 0
	for l <= h {
		mid := l + (h-l)/2
		if (*pq)[mid].msgID == searchedId {
			return true,mid, (*pq)[mid]
		} else if (*pq)[mid].msgID < searchedId {
			l = mid + 1
		} else {
			h = mid - 1
		}
	}

	return false,-1, nil
}

//Back retrieves the element with the biggest id or nil if list is empty
func (pq *SortedIndexList) Back() *IndexFileEntry {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[pq.Len()-1]
}

//Front retrieves the element with the smallest id or nil if list is empty
func (pq *SortedIndexList) Front() *IndexFileEntry {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[0]
}

//Front retrieves the element at the given index or nil if position is incorrect or list is empty
func (pq *SortedIndexList) Get(pos int) *IndexFileEntry {
	if pq.Len() ==0 ||  pos < 0 || pos >= pq.Len()  {
		return nil
	}
	return (*pq)[pos]
}

//TODO remove after usage.(ONly for testing )
func (pq *SortedIndexList) PrintPq() {
	for i := 0; i < pq.Len(); i++ {
		messageStoreLogger.WithFields(logrus.Fields{
			"msgSize":   (*pq)[i].msgSize,
			"msgId":     (*pq)[i].msgID,
			"msgOffset": (*pq)[i].messageOffset,
			"filename":  (*pq)[i].filename,
		}).Info("Printing element")
	}
}

func createIndexPriorityQueue(size int) *SortedIndexList {
	pq := make(SortedIndexList, 0)
	return &pq
}
