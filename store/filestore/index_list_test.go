package filestore

import (
	"math/rand"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_SortedListSanity(t *testing.T) {

	a := assert.New(t)
	pq := newList(1000)

	generatedIds := make([]uint64, 0, 11)

	for i := 0; i < 11; i++ {
		msgID := uint64(rand.Intn(50))
		generatedIds = append(generatedIds, msgID)

		entry := &Index{
			size:   3,
			id:     uint64(msgID),
			offset: 128,
		}

		pq.insert(entry)
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
		found, pos, _, foundEntry := pq.search(id)
		a.True(found)
		a.Equal(foundEntry.id, id)
		a.True(pos >= 0 && pos <= len(generatedIds))
	}

	logrus.WithField("generatedIds", generatedIds).Info("IdS")

	a.Equal(min, pq.front().id)
	a.Equal(max, pq.back().id)

	found, pos, bestIndex, foundEntry := pq.search(uint64(46))
	a.False(found, "Element should not be found since is a number greater than the random generated upper limit")
	a.Equal(pos, -1)
	a.Nil(foundEntry)
	logrus.WithField("bestIndexbestIndex", bestIndex).Info("bEST")

	a.Equal(pq.front().id, pq.get(0).id, "First element should contain the smallest element")
	a.Nil(pq.get(-1), "Trying to get an invalid index will return nil")

	pq.clear()
	a.Nil(pq.front())
	a.Nil(pq.back())

}
