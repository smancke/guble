package cluster

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"errors"
	"fmt"
	"io/ioutil"
)

const (
	eventChannelBufferSize = 1 << 4
)

type Config struct {
	ID      int
	Host    string
	Port    int
	Remotes []string
}

type Cluster struct {
	Config     *Config
	memberlist *memberlist.Memberlist
	eventC     chan memberlist.NodeEvent
}

func NewCluster(config *Config) *Cluster {
	cluster := &Cluster{
		Config: config,
		eventC: make(chan memberlist.NodeEvent, eventChannelBufferSize),
	}

	c := memberlist.DefaultLANConfig()
	c.Name = fmt.Sprintf("%d:%s:%d", config.ID, config.Host, config.Port)
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
	log.WithField("remotes", cluster.Config.Remotes).Debug("Starting Cluster")
	num, err := cluster.memberlist.Join(cluster.Config.Remotes)
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

func (cluster *Cluster) BroadcastMessage(message *ClusterMessage) {
	log.WithField("message", message).Debug("BroadcastMessage to cluster")
	//TODO Marian convert to byte array and invoke "cluster.broadcast"
	//encode the message and send it to
	bytes, err := message.EncodeMessage()
	if err != nil {
		logger.WithField("err", err).Error("Could not sent message")
	}
	log.WithFields(log.Fields{
		"nodeId":     cluster.Config.ID,
		"msgAsBytes": bytes,
	}).Debug("BroadcastMessage")

	cluster.broadcast(bytes)
}

func (cluster *Cluster) broadcast(msg []byte) {
	log.WithField("msg", msg).Debug("broadcast to cluster")
	for _, node := range cluster.memberlist.Members() {
		cluster.memberlist.SendToTCP(node, msg)

	}
}
