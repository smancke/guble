package store

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"container/heap"
	"github.com/Sirupsen/logrus"
	"github.com/smancke/guble/testutil"
	"math/rand"
)

func Test_Limit(t *testing.T){
	a := assert.New(t)
	pq := createIndexPriorityQueue()

	defer testutil.EnableDebugForMethod() ()

	for i := 0 ; i < 10; i++ {
		msgID :=   uint64(rand.Intn(14))
		entry := &IndexFileEntry{
			msgSize: 3,
			msgID: msgID,
			filename: "file",
			//index:i,
			messageOffset: 128,
		}
		heap.Push(pq, entry)
		logrus.WithFields(logrus.Fields{
			"msgId":msgID,
			//"index": ,
		}).Info("added elememnet")
	}


	pq.PrintPq()
	logrus.Info("----------------+++-------------------------------")
	pq.Clear()
	logrus.Info("----------------+++-------------------------------")

	entry := &IndexFileEntry{
		msgSize: 3,
		msgID: 21,
		filename: "file",
		//index:i,
		messageOffset: 128,
	}
	heap.Push(pq, entry)
	entry.msgID = 24
	heap.Push(pq, entry)
	pq.PrintPq()
	logrus.WithFields(logrus.Fields{
		"len": pq.Len(),
	}).Info("has 9")
	//pp, en := pq.GetIndexEntryFromID(9)
	//logrus.WithFields(logrus.Fields{
	//	"msgId": pp,
	//	"index": en,
	//}).Info("has 9")
	//
	//pp, en = pq.GetIndexEntryFromID(1)
	//
	//logrus.WithFields(logrus.Fields{
	//	"msgId": pp,
	//	"index": en,
	//}).Info("has 1")
	//
	//pp, en = pq.GetIndexEntryFromID(13)
	//logrus.WithFields(logrus.Fields{
	//	"msgId": pp,
	//	"index": en,
	//}).Info("has 13")
	//
	//
	//pp, en = pq.GetIndexEntryFromID(2)
	//logrus.WithFields(logrus.Fields{
	//	"msgId": pp,
	//	"index": en,
	//}).Info("has 2")
	//a.False(pp)
	//
	//tt:= pq.Peek()
	//logrus.WithFields(logrus.Fields{
	//	"msgId": tt.msgID,
	//	"index": tt.index,
	//}).Info("PEEK")

	//
	//for pq.Len() > 0 {
	//	item := heap.Pop(pq).(*IndexFileEntry)
	//	logrus.WithFields(logrus.Fields{
	//		"index": item.index,
	//		"msgID": item.msgId,
	//	}).Info("Poppign element")
	//}

	a.Nil(nil)
}
