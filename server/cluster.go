package server

import (
	"github.com/smancke/guble/protocol"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"fmt"
	"io/ioutil"
)

const (
	eventChannelBufferSize = 1 << 4
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
		eventC: make(chan memberlist.NodeEvent, eventChannelBufferSize),
	}

	c := memberlist.DefaultLANConfig()
	c.Name = fmt.Sprintf("%d:%s:%d", config.id, config.host, config.port)
	c.BindAddr = config.host
	c.BindPort = config.port

	c.Delegate = &ClusterDelegate{}

	c.Events = &ClusterEventDelegate{}

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

func (cluster *Cluster) Stop() error {
	return cluster.memberlist.Shutdown()
}

func (cluster *Cluster) BroadcastMessage(message protocol.Message) {
	logger.WithField("message", message).Debug("BroadcastMessage to cluster")
	//TODO Marian convert to byte array and invoke "cluster.broadcast"
}

func (cluster *Cluster) broadcast(msg []byte) {
	logger.Debug("broadcast to cluster")
	for _, node := range cluster.memberlist.Members() {
		cluster.memberlist.SendToTCP(node, msg)
	}
}
