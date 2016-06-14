package server

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"fmt"
	"io/ioutil"
)

const (
	channelBuffer = 1 << 8
)

type ClusterConfig struct {
	nodeID    int
	addr      string
	port      int
	nodesUrls []string
}

type Cluster struct {
	config     *ClusterConfig
	memberlist *memberlist.Memberlist
	eventC     chan memberlist.NodeEvent
}

func initCluster(config *ClusterConfig) *Cluster {
	cluster := &Cluster{
		config: config,
		eventC: make(chan memberlist.NodeEvent, channelBuffer),
	}

	c := memberlist.DefaultLANConfig()
	c.Name = fmt.Sprintf("%d:%s:%d", config.nodeID, config.addr, config.port)
	c.BindAddr = config.addr
	c.BindPort = config.port

	c.Delegate = &ClusterDelegate{}
	c.Events = &memberlist.ChannelEventDelegate{cluster.eventC}

	//TODO Cosmin temporarily disabling any logging from memberlist
	c.LogOutput = ioutil.Discard

	logger.Info("Creating memberlist")
	newMemberList, err := memberlist.Create(c)
	if err != nil {
		log.WithField("error", err).Fatal("Unexpected fatal error when creating the memberlist")
	}

	num, err := newMemberList.Join(config.nodesUrls)
	if num == 0 || err != nil {
		log.WithField("error", err).Fatal("Unexpected fatal error when node wanted to join the cluster")
	}
	cluster.memberlist = newMemberList
	return cluster
}

func (cluster *Cluster) Stop() error {
	return cluster.memberlist.Shutdown()
}
