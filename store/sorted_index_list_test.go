package store

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/smancke/guble/testutil"
	"math/rand"
	"github.com/Sirupsen/logrus"
)
func Test_Limit(t *testing.T){

	a := assert.New(t)
	pq := createIndexPriorityQueue(1000)

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
		pq.Insert(entry)
		//logrus.WithFields(logrus.Fields{
		//	"msgId":msgID,
		//	//"index": ,
		//}).Info("added elememnet")
	}


	logrus.Info("----------------+++-------------------------------")
	pq.PrintPq()
	//pq.Clear()
	//logrus.Info("----------------+++-------------------------------")
	//
	//entry := &IndexFileEntry{
	//	msgSize: 3,
	//	msgID: 21,
	//	filename: "file",
	//	//index:i,
	//	messageOffset: 128,
	//}
	//heap.Push(pq, entry)
	//entry.msgID = 24
	//heap.Push(pq, entry)
	//pq.PrintPq()
	//logrus.WithFields(logrus.Fields{
	//	"len": pq.Len(),
	//}).Info("has 9")
	pp,pos, en := pq.GetIndexEntryFromID(9)
	logrus.WithFields(logrus.Fields{
		"msgId": pp,
		"index": en,
		"pos": pos,
	}).Info("has 9")
	////
	////pp, en = pq.GetIndexEntryFromID(1)
	////
	////logrus.WithFields(logrus.Fields{
	////	"msgId": pp,
	////	"index": en,
	////}).Info("has 1")
	////
	////pp, en = pq.GetIndexEntryFromID(13)
	////logrus.WithFields(logrus.Fields{
	////	"msgId": pp,
	////	"index": en,
	////}).Info("has 13")
	////
	////
	////pp, en = pq.GetIndexEntryFromID(2)
	////logrus.WithFields(logrus.Fields{
	////	"msgId": pp,
	////	"index": en,
	////}).Info("has 2")
	////a.False(pp)
	////
	tt:= pq.Back()
	logrus.WithFields(logrus.Fields{
		"msgId": tt.msgID,
	}).Info("PEEK")
	//
	////
	////for pq.Len() > 0 {
	////	item := heap.Pop(pq).(*IndexFileEntry)
	////	logrus.WithFields(logrus.Fields{
	////		"index": item.index,
	////		"msgID": item.msgId,
	////	}).Info("Poppign element")
	////}

	a.Nil(nil)
}
