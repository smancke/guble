package cluster

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"github.com/smancke/guble/protocol"

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
	memberlistConfig.Delegate = &delegate{}
	memberlistConfig.Events = &eventDelegate{}

	//TODO Cosmin temporarily disabling any logging from memberlist
	memberlistConfig.LogOutput = ioutil.Discard

	memberlist, err := memberlist.Create(memberlistConfig)
	if err != nil {
		logger.WithField("error", err).Fatal("Fatal error when creating the internal memberlist of the cluster")
	}
	c.memberlist = memberlist
	return c
}

func (cluster *Cluster) Start() error {
	logger.WithField("remotes", cluster.Config.Remotes).Debug("Starting Cluster")
	num, err := cluster.memberlist.Join(cluster.Config.Remotes)
	if err != nil {
		logger.WithField("error", err).Error("Error when this node wanted to join the cluster")
		return err
	}
	if num == 0 {
		errorMessage := "No remote hosts were successfuly contacted when this node wanted to join the cluster"
		logger.Error(errorMessage)
		return errors.New(errorMessage)
	}
	return nil
}

func (cluster *Cluster) Stop() error {
	return cluster.memberlist.Shutdown()
}

func (cluster *Cluster) BroadcastString(sMessage *string) {
	logger.WithField("string", sMessage).Debug("BroadcastString")
	cMessage := &message{
		NodeID: cluster.Config.ID,
		Type:   STRING_BODY_MESSAGE,
		Body:   []byte(*sMessage),
	}
	cluster.broadcastClusterMessage(cMessage)
}

func (cluster *Cluster) BroadcastMessage(pMessage *protocol.Message) {
	logger.WithField("message", pMessage).Debug("BroadcastMessage")
	cMessage := &message{
		NodeID: cluster.Config.ID,
		Type:   MESSAGE,
		Body:   pMessage.Bytes(),
	}
	cluster.broadcastClusterMessage(cMessage)
}

func (cluster *Cluster) broadcastClusterMessage(cMessage *message) {
	logger.WithField("clusterMessage", cMessage).Debug("broadcastClusterMessage")
	cMessageBytes, err := cMessage.encode()
	if err != nil {
		logger.WithField("err", err).Error("Could not encode and send clusterMessage")
	}
	logger.WithFields(log.Fields{
		"nodeId":                cMessage.NodeID,
		"clusterMessageAsBytes": cMessageBytes,
	}).Debug("broadcastClusterMessage bytes")

	for _, node := range cluster.memberlist.Members() {
		if cluster.memberlist.LocalNode().Name != node.Name {
			logger.WithField("nodeName", node.Name).Debug("Sending MSG to node")
			cluster.memberlist.SendToTCP(node, cMessageBytes)
		}
	}
}
