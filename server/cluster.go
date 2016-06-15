package server

import (
	"github.com/smancke/guble/protocol"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"errors"
	"fmt"
	"io/ioutil"
)

const (
	eventChannelBufferSize = 1 << 4
)

type ClusterConfig struct {
	Id      int
	Host    string
	Port    int
	Remotes []string
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
	c.Name = fmt.Sprintf("%d:%s:%d", config.Id, config.Host, config.Port)
	c.BindAddr = config.Host
	c.BindPort = config.Port

	c.Delegate = &ClusterDelegate{}
	c.Events = &ClusterEventDelegate{}

	//TODO Cosmin temporarily disabling any logging from memberlist
	c.LogOutput = ioutil.Discard

	memberlist, err := memberlist.Create(c)
	if err != nil {
		log.WithField("error", err).Fatal("Fatal error when creating the internal memberlist of the cluster")
	}
	cluster.memberlist = memberlist
	return cluster
}

func (cluster *Cluster) Start() error {
	log.WithField("remotes", cluster.config.Remotes).Debug("Starting Cluster")
	num, err := cluster.memberlist.Join(cluster.config.Remotes)
	if err != nil {
		log.WithField("error", err).Error("Error when this node wanted to join the cluster")
		return err
	}
	if num == 0 {
		errorMessage := "No remote hosts were successfuly contacted when this node wanted to join the cluster"
		log.Error(errorMessage)
		return errors.New(errorMessage)
	}
	return nil
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
