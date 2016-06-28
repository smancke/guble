package store

import "container/heap"


// An Item is something we manage in a priority queue.
type IndexFileEntry struct {
	msgSize    uint32 // The value of the item; arbitrary.
	msgId uint64    // The priority of the item in the queue.
			// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.

	filename string

	messageOffset uint64
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*IndexFileEntry

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].msgId > pq[j].msgId
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

//// update modifies the priority and value of an Item in the queue.
//func (pq *PriorityQueue) update(item *IndexFileEntry, value string, priority int) {
//	item.value = value
//	item.msgId = priority
//	heap.Fix(pq, item.index)
//}

func createIndexPriorityQueur() *PriorityQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	return &pq

}
