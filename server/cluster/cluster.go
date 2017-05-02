package cluster

import (
	"io/ioutil"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/store"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"

	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

var (
	ErrNodeNotFound = errors.New("Node not found.")
)

// Config is a struct used by the local node when creating and running the guble cluster
type Config struct {
	ID                   uint8
	Host                 string
	Port                 int
	Remotes              []*net.TCPAddr
	HealthScoreThreshold int
}

// router interface specify only the methods we require in cluster from the Router
// router is an interface used for handling messages in cluster.
// It is logically connected to the router.Router interface, by reusing the same func signature.
type router interface {
	HandleMessage(message *protocol.Message) error
	MessageStore() (store.MessageStore, error)
}

// Cluster is a struct for managing the `local view` of the guble cluster, as seen by a node.
type Cluster struct {
	// Pointer to a Config struct, based on which the Cluster node is created and runs.
	Config *Config

	// Router is used for dispatching messages received by this node.
	// Should be set after the node is created with New(), and before Start().
	Router router

	name       string
	memberlist *memberlist.Memberlist
	broadcasts [][]byte

	numJoins   int
	numLeaves  int
	numUpdates int

	synchronizer *synchronizer
}

//New returns a new instance of the cluster, created using the given Config.
func New(config *Config) (*Cluster, error) {
	c := &Cluster{
		Config: config,
		name:   fmt.Sprintf("%d", config.ID),
	}

	memberlistConfig := memberlist.DefaultLANConfig()
	memberlistConfig.Name = c.name
	memberlistConfig.BindAddr = config.Host
	memberlistConfig.BindPort = config.Port

	//TODO Cosmin temporarily disabling any logging from memberlist, we might want to enable it again using logrus?
	memberlistConfig.LogOutput = ioutil.Discard

	ml, err := memberlist.Create(memberlistConfig)
	if err != nil {
		logger.WithField("error", err).Error("Error when creating the internal memberlist of the cluster")
		return nil, err
	}
	c.memberlist = ml
	memberlistConfig.Delegate = c
	memberlistConfig.Conflict = c
	memberlistConfig.Events = c

	return c, nil
}

// Start the cluster module.
func (cluster *Cluster) Start() error {
	logger.WithField("remotes", cluster.Config.Remotes).Debug("Starting Cluster")

	if cluster.Router == nil {
		errorMessage := "There should be a valid Router already set-up"
		logger.Error(errorMessage)
		return errors.New(errorMessage)
	}

	synchronizer, err := newSynchronizer(cluster)
	if err != nil {
		logger.WithError(err).Error("Error creating cluster synchronizer")
		return err
	}
	cluster.synchronizer = synchronizer

	num, err := cluster.memberlist.Join(cluster.remotesAsStrings())
	if err != nil {
		logger.WithField("error", err).Error("Error when this node wanted to join the cluster")
		return err
	}
	if num == 0 {
		errorMessage := "No remote hosts were successfully contacted when this node wanted to join the cluster"
		logger.WithField("remotes", cluster.remotesAsStrings()).Error(errorMessage)
		return errors.New(errorMessage)
	}

	logger.Debug("Started Cluster")

	return nil
}

// Stop the cluster module.
func (cluster *Cluster) Stop() error {
	if cluster.synchronizer != nil {
		close(cluster.synchronizer.stopC)
	}
	cluster.memberlist.Leave(time.Second)
	return cluster.memberlist.Shutdown()
}

// Check returns a non-nil error if the health status of the cluster (as seen by this node) is not perfect.
func (cluster *Cluster) Check() error {
	if healthScore := cluster.memberlist.GetHealthScore(); healthScore > cluster.Config.HealthScoreThreshold {
		errorMessage := "Cluster Health Score is not perfect"
		logger.WithField("healthScore", healthScore).Error(errorMessage)
		return errors.New(errorMessage)
	}
	return nil
}

// newMessage returns a *message to be used in broadcasting or sending to a node
func (cluster *Cluster) newMessage(t messageType, body []byte) *message {
	return &message{
		NodeID: cluster.Config.ID,
		Type:   t,
		Body:   body,
	}
}

func (cluster *Cluster) newEncoderMessage(t messageType, entity encoder) (*message, error) {
	body, err := entity.encode()
	if err != nil {
		return nil, err
	}
	return cluster.newMessage(t, body), nil
}

// BroadcastString broadcasts a string to all the other nodes in the guble cluster
func (cluster *Cluster) BroadcastString(sMessage *string) error {
	logger.WithField("string", sMessage).Debug("BroadcastString")
	cMessage := &message{
		NodeID: cluster.Config.ID,
		Type:   mtStringMessage,
		Body:   []byte(*sMessage),
	}
	return cluster.broadcastClusterMessage(cMessage)
}

// BroadcastMessage broadcasts a guble-protocol-message to all the other nodes in the guble cluster.
func (cluster *Cluster) BroadcastMessage(pMessage *protocol.Message) error {
	logger.WithField("message", pMessage).Debug("BroadcastMessage")
	cMessage := &message{
		NodeID: cluster.Config.ID,
		Type:   mtGubleMessage,
		Body:   pMessage.Encode(),
	}
	return cluster.broadcastClusterMessage(cMessage)
}

func (cluster *Cluster) broadcastClusterMessage(cMessage *message) error {
	if cMessage == nil {
		errorMessage := "Could not broadcast a nil cluster-message"
		logger.Error(errorMessage)
		return errors.New(errorMessage)
	}

	cMessageBytes, err := cMessage.encode()
	if err != nil {
		logger.WithError(err).Error("Could not encode and broadcast cluster-message")
		return err
	}

	for _, node := range cluster.memberlist.Members() {
		if cluster.name == node.Name {
			continue
		}
		go cluster.sendToNode(node, cMessageBytes)
	}
	return nil
}

func (cluster *Cluster) sendToNode(node *memberlist.Node, msgBytes []byte) error {
	logger.WithFields(log.Fields{
		"node": cluster.Config.ID,
		"to":   node.Name,
	}).Debug("Sending cluster-message to a node")

	err := cluster.memberlist.SendToTCP(node, msgBytes)
	if err != nil {
		logger.WithFields(log.Fields{
			"err":  err,
			"node": node,
		}).Error("Error sending cluster-message to a node")

		return err
	}

	return nil
}

func (cluster *Cluster) sendMessageToNode(node *memberlist.Node, cmsg *message) error {
	logger.WithField("node", node.Name).Debug("Sending message to a node")

	bytes, err := cmsg.encode()
	if err != nil {
		logger.WithError(err).Error("Could not encode and broadcast cluster-message")
		return err
	}

	if err = cluster.memberlist.SendToTCP(node, bytes); err != nil {
		logger.WithField("node", node.Name).WithError(err).Error("Error send message to node")
		return err
	}

	return nil
}

func (cluster *Cluster) sendMessageToNodeID(nodeID uint8, cmsg *message) error {
	node := cluster.GetNodeByID(nodeID)
	if node == nil {
		return ErrNodeNotFound
	}

	return cluster.sendMessageToNode(node, cmsg)
}

func (cluster *Cluster) GetNodeByID(id uint8) *memberlist.Node {
	name := strconv.FormatUint(uint64(id), 10)
	for _, node := range cluster.memberlist.Members() {
		if node.Name == name {
			return node
		}
	}
	return nil
}

func (cluster *Cluster) remotesAsStrings() (strings []string) {
	log.WithField("Remotes", cluster.Config.Remotes).Debug("Cluster remotes")
	for _, remote := range cluster.Config.Remotes {
		strings = append(strings, remote.IP.String()+":"+strconv.Itoa(remote.Port))
	}
	return
}
