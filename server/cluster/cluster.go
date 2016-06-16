package cluster

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"errors"
	"fmt"
	"github.com/smancke/guble/protocol"
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

func (cluster *Cluster) BroadcastString(sMessage *string) {
	log.WithField("string", sMessage).Debug("BroadcastString")
	cMessage := &clusterMessage{
		NodeID: cluster.Config.ID,
		Type:   STRING_BODY_MESSAGE,
		Body:   []byte(*sMessage),
	}
	cluster.broadcastClusterMessage(cMessage)
}

func (cluster *Cluster) BroadcastMessage(pMessage *protocol.Message) {
	log.WithField("message", pMessage).Debug("BroadcastMessage")
	cMessage := &clusterMessage{
		NodeID: cluster.Config.ID,
		Type:   MESSAGE,
		Body:   pMessage.Bytes(),
	}
	cluster.broadcastClusterMessage(cMessage)
}

func (cluster *Cluster) broadcastClusterMessage(cMessage *clusterMessage) {
	log.WithField("clusterMessage", cMessage).Debug("BroadcastMessage to cluster")
	bytes, err := cMessage.EncodeMessage()
	if err != nil {
		logger.WithField("err", err).Error("Could not send clusterMessage")
	}
	log.WithFields(log.Fields{
		"nodeId":         cMessage.NodeID,
		"messageAsBytes": bytes,
	}).Debug("broadcastClusterMessage")

	cluster.broadcast(bytes)
}

func (cluster *Cluster) broadcast(msg []byte) {
	log.WithField("msg", msg).Debug("broadcast to cluster")
	for _, node := range cluster.memberlist.Members() {
		cluster.memberlist.SendToTCP(node, msg)

	}
}
