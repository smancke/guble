package server

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"fmt"
	"io/ioutil"
)

const (
	eventChannelBuffer = 1 << 4
)

type ClusterConfig struct {
	id                   int
	host                 string
	port                 int
	remoteHostsWithPorts []string
}

type Cluster struct {
	config     *ClusterConfig
	memberlist *memberlist.Memberlist
	eventC     chan memberlist.NodeEvent
}

func NewCluster(config *ClusterConfig) *Cluster {
	cluster := &Cluster{
		config: config,
		eventC: make(chan memberlist.NodeEvent, eventChannelBuffer),
	}

	c := memberlist.DefaultLANConfig()
	c.Name = fmt.Sprintf("%d:%s:%d", config.id, config.host, config.port)
	c.BindAddr = config.host
	c.BindPort = config.port

	c.Delegate = &ClusterDelegate{}
	c.Events = &memberlist.ChannelEventDelegate{cluster.eventC}

	//TODO Cosmin temporarily disabling any logging from memberlist
	c.LogOutput = ioutil.Discard

	memberlist, err := memberlist.Create(c)
	if err != nil {
		log.WithField("error", err).Fatal("Unexpected fatal error when creating the memberlist")
	}

	num, err := memberlist.Join(config.remoteHostsWithPorts)
	if num == 0 || err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"num":   num,
		}).Fatal("Unexpected fatal error when node wanted to join the cluster")
	}
	cluster.memberlist = memberlist
	return cluster
}

func (cluster *Cluster) Broadcast(msg []byte) {
	log.WithField("msg", msg).Debug("Broadcasting message to cluster")
	for _, node := range cluster.memberlist.Members() {
		cluster.memberlist.SendToTCP(node, msg)
	}
}

func (cluster *Cluster) Stop() error {
	return cluster.memberlist.Shutdown()
}
