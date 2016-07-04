package store

import (
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"github.com/Sirupsen/logrus"
)

func Test_SortedListSanity(t *testing.T) {

	a := assert.New(t)
	pq := createIndexPriorityQueue(1000)

	defer testutil.EnableDebugForMethod()()

	generatedIds := make([]uint64, 0, 11)

	for i := 0; i < 11; i++ {
		msgID := uint64(rand.Intn(50))
		generatedIds = append(generatedIds, msgID)

		entry := &FetchEntry{
			size:  3,
			messageId:    uint64(msgID),
			offset: 128,
		}
		pq.Insert(entry)
	}
	min := uint64(200)
	max := uint64(0)

	for _, id := range generatedIds {
		if max < id {
			max = id
		}
		if min > id {
			min = id
		}
		found, pos,_, foundEntry := pq.GetIndexEntryFromID(id)
		a.True(found)
		a.Equal(foundEntry.messageId, id)
		a.True(pos >= 0 && pos <= len(generatedIds))
	}

	logrus.WithField("generatedIds", generatedIds).Info("IdS")

	a.Equal(min, pq.Front().messageId)
	a.Equal(max, pq.Back().messageId)

	found, pos,bestIndexbestIndex, foundEntry := pq.GetIndexEntryFromID(uint64(46))
	a.False(found, "Element should not be found since is a number greater than the random generated upper limit")
	a.Equal(pos, -1)
	a.Nil(foundEntry)
	logrus.WithField("bestIndexbestIndex", bestIndexbestIndex).Info("bEST")


	a.Equal(pq.Front().messageId,pq.Get(0).messageId,"First element should contain the smallest element")
	a.Nil(pq.Get(-1), "Trying to get an invalid index will return nil")

	pq.Clear()
	a.Nil(pq.Front())
	a.Nil(pq.Back())

}
