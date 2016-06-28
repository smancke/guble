package store

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"time"
	"container/heap"
	"github.com/Sirupsen/logrus"
	"github.com/smancke/guble/testutil"
)

func Test_Limit(t *testing.T){
	a := assert.New(t)
	pq := createIndexPriorityQueur()

	defer testutil.EnableDebugForMethod() ()

	for i := 0 ; i < 130; i++ {
		msgID :=   uint64(time.Now().Unix() %128)
		entry := &IndexFileEntry{
			msgSize: 3,
			msgId: msgID,
			filename: "file",
			index:i,
			messageOffset: 128,
		}
		heap.Push(pq, entry)
		logrus.WithFields(logrus.Fields{
			"msgId":msgID,
			"index": i,
		}).Info("added elememnet")
	}


	for pq.Len() > 0 {
		item := heap.Pop(pq).(*IndexFileEntry)
		logrus.WithFields(logrus.Fields{
			"index": item.index,
			"msgID": item.msgId,
		})                         .Info("Poppign element")
	}

	a.Nil(nil)
}
