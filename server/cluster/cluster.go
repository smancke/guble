package cluster

import (
	"github.com/smancke/guble/protocol"

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

type MessageHandler interface {
	HandleMessage(message *protocol.Message) error
}

type Cluster struct {
	Config         *Config
	memberlist     *memberlist.Memberlist
	MessageHandler MessageHandler
	broadcasts     [][]byte
}

func New(config *Config) *Cluster {
	c := &Cluster{Config: config}

	memberlistConfig := memberlist.DefaultLANConfig()
	memberlistConfig.Name = fmt.Sprintf("%d:%s:%d", config.ID, config.Host, config.Port)
	memberlistConfig.BindAddr = config.Host
	memberlistConfig.BindPort = config.Port
	memberlistConfig.Events = &eventDelegate{}

	//TODO Cosmin temporarily disabling any logging from memberlist, we might want to enable it again using logrus?
	memberlistConfig.LogOutput = ioutil.Discard

	memberlist, err := memberlist.Create(memberlistConfig)
	if err != nil {
		logger.WithField("error", err).Fatal("Fatal error when creating the internal memberlist of the cluster")
	}
	c.memberlist = memberlist
	memberlistConfig.Delegate = c
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

func (cluster *Cluster) NotifyMsg(msg []byte) {
	logger.WithField("msgAsBytes", msg).Debug("NotifyMsg")

	cmsg, err := decode(msg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding of cluster message failed")
		return
	}
	logger.WithFields(log.Fields{
		"senderNodeID": cmsg.NodeID,
		"type":         cmsg.Type,
		"body":         string(cmsg.Body),
	}).Debug("NotifyMsg: Received cluster message")

	if cluster.MessageHandler != nil && cmsg.Type == GUBLE_MESSAGE {
		gmsg, err := protocol.ParseMessage(cmsg.Body)
		if err != nil {
			logger.WithField("err", err).Error("Parsing of guble-message contained in cluster-message failed")
			return
		}
		cluster.MessageHandler.HandleMessage(gmsg)
	}
}

func (cluster *Cluster) GetBroadcasts(overhead, limit int) [][]byte {
	b := cluster.broadcasts
	cluster.broadcasts = nil
	return b
}

func (cluster *Cluster) NodeMeta(limit int) []byte {
	return nil
}

func (cluster *Cluster) LocalState(join bool) []byte {
	return nil
}

func (cluster *Cluster) MergeRemoteState(s []byte, join bool) {
}

func (cluster *Cluster) BroadcastString(sMessage *string) error {
	logger.WithField("string", sMessage).Debug("BroadcastString")
	cMessage := &message{
		NodeID: cluster.Config.ID,
		Type:   STRING_BODY_MESSAGE,
		Body:   []byte(*sMessage),
	}
	return cluster.broadcastClusterMessage(cMessage)
}

func (cluster *Cluster) BroadcastMessage(pMessage *protocol.Message) error {
	logger.WithField("message", pMessage).Debug("BroadcastMessage")
	cMessage := &message{
		NodeID: cluster.Config.ID,
		Type:   GUBLE_MESSAGE,
		Body:   pMessage.Bytes(),
	}
	return cluster.broadcastClusterMessage(cMessage)
}

func (cluster *Cluster) broadcastClusterMessage(cMessage *message) error {
	logger.WithField("clusterMessage", cMessage).Debug("broadcastClusterMessage")
	cMessageBytes, err := cMessage.encode()
	if err != nil {
		logger.WithField("err", err).Error("Could not encode and send clusterMessage")
		return err
	}
	logger.WithFields(log.Fields{
		"nodeId":                cMessage.NodeID,
		"clusterMessageAsBytes": cMessageBytes,
	}).Debug("broadcastClusterMessage bytes")

	for _, node := range cluster.memberlist.Members() {
		if cluster.memberlist.LocalNode().Name != node.Name {
			logger.WithField("nodeName", node.Name).Debug("Sending cluster-message to a guble node")
			err := cluster.memberlist.SendToTCP(node, cMessageBytes)
			if err != nil {
				logger.WithFields(log.Fields{
					"err":  err,
					"node": node,
				}).Error("Error sending message to node")
				return err
			}
		}
	}
	return nil
}
