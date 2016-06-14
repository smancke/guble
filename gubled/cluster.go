package gubled

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"fmt"
	"io/ioutil"
	"time"
)

type GubleDelegate struct {
	msgs       [][]byte
	broadcasts [][]byte
}

func (gd *GubleDelegate) NotifyMsg(msg []byte) {
	log.WithField("message", string(msg)).Debug("NotifyMsg")
	cp := make([]byte, len(msg))
	copy(cp, msg)
	gd.msgs = append(gd.msgs, cp)
}

func (gd *GubleDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	log.WithFields(log.Fields{
		"overhead": overhead,
		"limit":    limit,
	}).Debug("NotifyMsg")
	b := gd.broadcasts
	gd.broadcasts = nil
	return b
}

func (gd *GubleDelegate) NodeMeta(limit int) []byte {
	return nil
}

func (gd *GubleDelegate) LocalState(join bool) []byte {
	return nil
}

func (gd *GubleDelegate) MergeRemoteState(s []byte, join bool) {
}

func BenchmarkCluster(num int, timeoutForAllJoins time.Duration, lowestPort int) {
	startTime := time.Now()
	var nodes []*memberlist.Memberlist
	eventC := make(chan memberlist.NodeEvent, num)
	addr := "127.0.0.1"
	var firstMemberName string
	for i := 0; i < num; i++ {
		c := memberlist.DefaultLANConfig()
		port := lowestPort + i
		c.Name = fmt.Sprintf("%s:%d", addr, port)
		c.BindAddr = addr
		c.BindPort = port
		c.ProbeInterval = 20 * time.Millisecond
		c.ProbeTimeout = 100 * time.Millisecond
		c.GossipInterval = 20 * time.Millisecond
		c.PushPullInterval = 200 * time.Millisecond
		c.Delegate = &GubleDelegate{}

		//TODO Cosmin temporarily disabling any logging from memberlist
		c.LogOutput = ioutil.Discard

		if i == 0 {
			c.Events = &memberlist.ChannelEventDelegate{eventC}
			firstMemberName = c.Name
		}

		newMemberList, err := memberlist.Create(c)
		if err != nil {
			log.WithField("error", err).Fatal("Unexpected error when creating the memberlist")
		}
		nodes = append(nodes, newMemberList)
		defer newMemberList.Shutdown()

		if i >= 0 {
			num, err := newMemberList.Join([]string{firstMemberName})
			if num == 0 || err != nil {
				log.WithField("error", err).Fatal("Unexpected fatal error when node wanted to join the cluster")
			}
		}
	}

	breakTimer := time.After(timeoutForAllJoins)
	numJoins := 0
WAIT:
	for {
		select {
		case e := <-eventC:
			l := log.WithFields(log.Fields{
				"node":         *e.Node,
				"eventCounter": numJoins,
				"numMembers":   nodes[0].NumMembers(),
			})
			if e.Event == memberlist.NodeJoin {
				l.Info("Node join")
				numJoins++
				if numJoins == num {
					l.Info("All nodes joined")
					break WAIT
				}
			} else {
				l.Info("Node leave")
			}
		case <-breakTimer:
			break WAIT
		}
	}

	if numJoins != num {
		log.WithFields(log.Fields{
			"joinCounter": numJoins,
			"num":         num,
		}).Error("Timeout before completing all joins")
	}

	convergence := false
	for !convergence {
		convergence = true
		for idx, node := range nodes {
			numSeenByNode := node.NumMembers()
			if numSeenByNode != num {
				log.WithFields(log.Fields{
					"index":    idx,
					"expected": num,
					"actual":   numSeenByNode,
				}).Debug("Wrong number of nodes")
				convergence = false
				break
			}
		}
	}
	endTime := time.Now()
	if numJoins == num {
		log.WithField("durationSeconds", endTime.Sub(startTime).Seconds()).Info("Cluster convergence reached")
	}

	for senderID, node := range nodes {
		for receiverID, member := range node.Members() {
			message := fmt.Sprintf("Hello from %v to %v !", senderID, receiverID)
			log.WithField("message", message).Debug("SendToTCP")
			node.SendToTCP(member, []byte(message))
		}
	}
}
