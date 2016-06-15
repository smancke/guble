package cluster

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"errors"
	"fmt"
	"io/ioutil"
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
}

func NewCluster(config *Config) *Cluster {
	c := &Cluster{Config: config}

	memberlistConfig := memberlist.DefaultLANConfig()
	memberlistConfig.Name = fmt.Sprintf("%d:%s:%d", config.ID, config.Host, config.Port)
	memberlistConfig.BindAddr = config.Host
	memberlistConfig.BindPort = config.Port
	memberlistConfig.Delegate = &Delegate{}
	memberlistConfig.Events = &EventDelegate{}

	//TODO Cosmin temporarily disabling any logging from memberlist
	memberlistConfig.LogOutput = ioutil.Discard

	memberlist, err := memberlist.Create(memberlistConfig)
	if err != nil {
		log.WithField("error", err).Fatal("Fatal error when creating the internal memberlist of the cluster")
	}
	c.memberlist = memberlist
	return c
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
