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

//INsert nu functioneaza corect ...nu creste corect dimensiunea array-ul...In test totusi merge ????
func (pq *SortedIndexList) Insert(newElement *IndexFileEntry) {
	messageStoreLogger.WithField("new_msgID", newElement.msgID).Info("Adding element")
	if pq.Len() == 0 {
		//messageStoreLogger.WithField("new_msgID", newElement.msgID).Info("Adding on empty list")
		*pq = append(*pq, newElement)
		return
	} else {
		currentPos := 0
		if (*pq)[currentPos].msgID >= newElement.msgID {
			*pq = append(SortedIndexList{newElement}, *pq...)
			//messageStoreLogger.WithField("new_msgID", newElement.msgID).Info("Adding on existing list on the firstPos")
			return
		} else {
			for i := 1; i < pq.Len()-1; i++ {
				if (*pq)[i].msgID > newElement.msgID {
					pq.insertAt(i, newElement)
					break
				}
			}
		}
	}

}

func (pq *SortedIndexList) insertAt(index int, newElement *IndexFileEntry) {
	*pq = append((*pq)[:index], append(SortedIndexList{newElement}, (*pq)[index:]...)...)
}

func (pq *SortedIndexList) Clear() {
	*pq = nil
	*pq = make(SortedIndexList, 0)

}

// GetIndexEntryFromID performs a binarySearch retrieving the index
func (pq *SortedIndexList) GetIndexEntryFromID(searchedId uint64) (bool, int ,*IndexFileEntry) {
	h := pq.Len()
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

func (pq *SortedIndexList) Back() *IndexFileEntry {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[pq.Len()-1]
}
func (pq *SortedIndexList) Front() *IndexFileEntry {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[0]
}

func (pq *SortedIndexList) Get(pos int) *IndexFileEntry {
	if pq.Len() < pos || pos < 0 {
		return nil
	}
	return (*pq)[pos]
}

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
