package filestore

import (
	"math/rand"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_SortedListSanity(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	list := newIndexList(1000)

	generatedIds := make([]uint64, 0, 11)

	for i := 0; i < 11; i++ {
		msgID := uint64(rand.Intn(50))
		generatedIds = append(generatedIds, msgID)

		entry := &index{
			size:   3,
			id:     uint64(msgID),
			offset: 128,
		}

		list.insert(entry)
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
		found, pos, _, foundEntry := list.search(id)
		a.True(found)
		a.Equal(foundEntry.id, id)
		a.True(pos >= 0 && pos <= len(generatedIds))
	}

	logrus.WithField("generatedIds", generatedIds).Info("IdS")

	a.Equal(min, list.front().id)
	a.Equal(max, list.back().id)

	found, pos, bestIndex, foundEntry := list.search(uint64(46))
	a.False(found, "Element should not be found since is a number greater than the random generated upper limit")
	a.Equal(pos, -1)
	a.Nil(foundEntry)
	logrus.WithField("bestIndex", bestIndex).Info("Seraching for closest position")

	a.Equal(list.front().id, list.get(0).id, "First element should contain the smallest element")
	a.Nil(list.get(-1), "Trying to get an invalid index will return nil")

	list.clear()
	a.Nil(list.front())
	a.Nil(list.back())

}
