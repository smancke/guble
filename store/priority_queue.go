package store

import (
	"container/heap"
	"github.com/Sirupsen/logrus"
)

// An Item is something we manage in a priority queue.
type IndexFileEntry struct {
	msgSize       uint32 // The value of the item; arbitrary.
	msgID         uint64 // The priority of the item in the queue.
			     // The index is needed by update and is maintained by the heap.Interface methods.
	index         int    // The index of the item in the heap.

	filename      string

	messageOffset uint64
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*IndexFileEntry

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest , not highest, priority so we use greater than here.
	return pq[i].msgID < pq[j].msgID
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*IndexFileEntry)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Clear()  {
	*pq = nil
	*pq = make(PriorityQueue, 0)
	heap.Init(pq)
	//return  pq
}

// GetIndexEntryFromID performs a binarySearch retrieving the index
func (pq *PriorityQueue) GetIndexEntryFromID(searchedId uint64) (bool, *IndexFileEntry) {
	h := pq.Len()
	l := 0
	for l <= h {
	     mid := l +(h-l)/2
		if (*pq)[mid].msgID == searchedId {
			return true, (*pq)[mid]
		}else if (*pq)[mid].msgID < searchedId {
		       l = mid +1
		}else {
			h = mid-1
		}
	}

	return false, nil
}

func (pq *PriorityQueue) Peek() *IndexFileEntry {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[pq.Len()-1]
}

func (pq *PriorityQueue) Get(pos int) *IndexFileEntry {
	if pq.Len() < pos || pos < 0{
		return nil
	}
	return (*pq)[pos]
}

func (pq *PriorityQueue) PrintPq() {
	for i:=0 ;i< pq.Len(); i++  {
		messageStoreLogger.WithFields(logrus.Fields{
			"msgSize": (*pq)[i].msgSize,
			"msgId": (*pq)[i].msgID,
			"index": (*pq)[i].index,
			"msgOffset": (*pq)[i].messageOffset,
			"filename": (*pq)[i].filename,
		}) .Debug("Printing element")
	}
}

func createIndexPriorityQueue() *PriorityQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	return &pq
}
