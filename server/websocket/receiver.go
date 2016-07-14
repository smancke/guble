package websocket

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
)

var errUnreadMsgsAvailable = errors.New("unread messages available")

// Receiver is a helper class, for managing a combined pull push on a topic.
// It is used for implementation of the + (receive) command in the guble protocol.
type Receiver struct {
	cancelC             chan bool
	sendC               chan []byte
	applicationId       string
	router              server.Router
	messageStore        store.MessageStore
	path                protocol.Path
	doFetch             bool
	doSubscription      bool
	startId             uint64
	maxCount            int
	lastSendId          uint64
	shouldStop          bool
	route               *server.Route
	enableNotifications bool
	userId              string
}

// NewReceiverFromCmd parses the info in the command
func NewReceiverFromCmd(
	applicationId string,
	cmd *protocol.Cmd,
	sendChannel chan []byte,
	router server.Router,
	userId string) (rec *Receiver, err error) {

	messageStore, err := router.MessageStore()
	if err != nil {
		return nil, err
	}

	rec = &Receiver{
		applicationId:       applicationId,
		sendC:               sendChannel,
		router:              router,
		messageStore:        messageStore,
		cancelC:             make(chan bool, 1),
		enableNotifications: true,
		userId:              userId,
	}
	if len(cmd.Arg) == 0 || cmd.Arg[0] != '/' {
		return nil, fmt.Errorf("command requires at least a path argument, but non given")
	}

	args := strings.SplitN(cmd.Arg, " ", 3)
	rec.path = protocol.Path(args[0])

	if len(args) > 1 {
		rec.doFetch = true
		rec.startId, err = strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("startid has to be empty or int, but was %q: %v", args[1], err)
		}
	}

	rec.doSubscription = true
	if len(args) > 2 {
		rec.doSubscription = false
		rec.maxCount, err = strconv.Atoi(args[2])
		if err != nil {
			return nil, fmt.Errorf("maxCount has to be empty or int, but was %q: %v", args[1], err)
		}
	}

	return rec, nil
}

// Start starts the receiver loop
func (rec *Receiver) Start() error {
	rec.shouldStop = false
	if rec.doFetch && !rec.doSubscription {
		go rec.fetchOnlyLoop()
	} else {
		go rec.subscriptionLoop()
	}
	return nil
}

func (rec *Receiver) subscriptionLoop() {
	for !rec.shouldStop {
		if rec.doFetch {

			if err := rec.fetch(); err != nil {
				logger.WithError(err).WithField("rec", rec).Error("Error while fetching subscription")
				rec.sendError(protocol.ERROR_INTERNAL_SERVER, err.Error())
				return
			}

			if err := rec.messageStore.DoInTx(rec.path.Partition(), rec.subscribeIfNoUnreadMessagesAvailable); err != nil {
				if err == errUnreadMsgsAvailable {
					//fmt.Printf(" errUnreadMsgsAvailable lastSendId=%v rec=%v \n", rec.lastSendId,rec)
					rec.startId = rec.lastSendId
					continue // fetch again
				} else {
					logger.WithError(err).WithField("recStartId", rec.startId).
						Error("Error while subscribeIfNoUnreadMessagesAvailable")
					rec.sendError(protocol.ERROR_INTERNAL_SERVER, err.Error())
					return
				}
			}
		}

		if !rec.doFetch {
			rec.subscribe()
		}
		rec.receiveFromSubscription()

		if !rec.shouldStop {
			//fmt.Printf(" router closed .. on msg: %v\n", rec.lastSendId)
			// the router kicked us out, because we are too slow for realtime listening,
			// so we setup parameters for fetching and closing the gap. Than we can subscribe again.
			rec.startId = rec.lastSendId
			rec.doFetch = true
		}
	}
}

func (rec *Receiver) subscribeIfNoUnreadMessagesAvailable(maxMessageId uint64) error {
	if maxMessageId > rec.lastSendId {
		return errUnreadMsgsAvailable
	}
	rec.subscribe()
	return nil
}

func (rec *Receiver) subscribe() {
	rec.route = server.NewRoute(string(rec.path), rec.applicationId, rec.userId, 3)
	_, err := rec.router.Subscribe(rec.route)
	if err != nil {
		rec.sendError(protocol.ERROR_SUBSCRIBED_TO, string(rec.path), err.Error())
	} else {
		rec.sendOK(protocol.SUCCESS_SUBSCRIBED_TO, string(rec.path))
	}
}

func (rec *Receiver) receiveFromSubscription() {
	for {
		select {
		case m, ok := <-rec.route.MessagesChannel():
			if !ok {

				logger.WithFields(log.Fields{
					"applicationId": rec.applicationId,
				}).Debug("Router closed the channel returning from subscription for")
				return
			}

			logger.WithFields(log.Fields{
				"applicationId":   rec.applicationId,
				"messageMetadata": m.Metadata(),
			}).Debug("Delivering message")

			if m.ID > rec.lastSendId {
				rec.lastSendId = m.ID
				rec.sendC <- m.Bytes()
			} else {
				logger.WithFields(log.Fields{
					"msgId": m.ID,
				}).Debug("Message already sent to client. Dropping message.")
			}
		case <-rec.cancelC:
			rec.shouldStop = true
			rec.router.Unsubscribe(rec.route)
			rec.route = nil
			rec.sendOK(protocol.SUCCESS_CANCELED, string(rec.path))
			return
		}
	}
}

func (rec *Receiver) fetchOnlyLoop() {
	err := rec.fetch()
	if err != nil {
		logger.WithError(err).WithField("rec", rec).Error("Error while fetching")
		rec.sendError(protocol.ERROR_INTERNAL_SERVER, err.Error())
	}
}

func (rec *Receiver) fetch() error {
	fetch := store.FetchRequest{
		Partition: rec.path.Partition(),
		MessageC:  make(chan store.FetchedMessage, 3),
		ErrorC:    make(chan error),
		StartC:    make(chan int),
		Prefix:    []byte(rec.path),
		Count:     rec.maxCount,
	}

	if rec.startId >= 0 {
		fetch.Direction = 1
		fetch.StartID = uint64(rec.startId)
		if rec.maxCount == 0 {
			fetch.Count = math.MaxInt32
		}
	} else {
		fetch.Direction = 1
		maxId, err := rec.messageStore.MaxMessageID(rec.path.Partition())
		if err != nil {
			return err
		}

		fetch.StartID = maxId + 1 + uint64(rec.startId)
		if rec.maxCount == 0 {
			fetch.Count = -1 * int(rec.startId)
		}
	}
	maxId, _ := rec.messageStore.MaxMessageID(rec.path.Partition())
	logger.WithFields(log.Fields{
		"rec.StartID": rec.startId,
		"fetch.StartID": fetch.StartID,
		"fetch.count":  fetch.Count,
		"maxID": maxId,
		"partition": rec.path.Partition(),
	}).Info("!Fetching in receiver")

	rec.messageStore.Fetch(fetch)

	for {
		select {
		case numberOfResults := <-fetch.StartC:
			rec.sendOK(protocol.SUCCESS_FETCH_START, fmt.Sprintf("%v %v", rec.path, numberOfResults))
		case msgAndID, open := <-fetch.MessageC:
			if !open {
				rec.sendOK(protocol.SUCCESS_FETCH_END, string(rec.path))
				return nil
			}
			logger.WithFields(log.Fields{
				"msgId": msgAndID.ID,
				"msg":   string(msgAndID.Message),
			}).Debug("Reply sent")

			rec.lastSendId = msgAndID.ID
			rec.sendC <- msgAndID.Message
		case err := <-fetch.ErrorC:
			return err
		case <-rec.cancelC:
			rec.shouldStop = true
			rec.sendOK(protocol.SUCCESS_CANCELED, string(rec.path))
			// TODO implement cancellation in message store
			return nil
		}
	}
}

// Stop stops/cancels the receiver
func (rec *Receiver) Stop() error {
	rec.cancelC <- true
	return nil
}

func (rec *Receiver) sendError(name string, argPattern string, params ...interface{}) {
	notificationMessage := &protocol.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: true,
	}
	rec.sendC <- notificationMessage.Bytes()
}

func (rec *Receiver) sendOK(name string, argPattern string, params ...interface{}) {
	if rec.enableNotifications {
		notificationMessage := &protocol.NotificationMessage{
			Name:    name,
			Arg:     fmt.Sprintf(argPattern, params...),
			IsError: false,
		}
		rec.sendC <- notificationMessage.Bytes()
	}
}
