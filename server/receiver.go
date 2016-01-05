package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var unread_messages_available = errors.New("unread messages available")

// A receiver is a helper class, for managing a combined pull push on a topic.
// It is used for implementation of the + (receive) command in the gubble protocol.
type Receiver struct {
	cancelChannel       chan bool
	sendChannel         chan []byte
	applicationId       string
	messageSouce        PubSubSource
	messageStore        store.MessageStore
	path                guble.Path
	doFetch             bool
	doSubscription      bool
	startId             int64
	maxCount            int
	lastSendId          uint64
	shouldStop          bool
	route               *Route
	enableNotifications bool
}

// Parses the info in the command
func NewReceiverFromCmd(applicationId string, cmd *guble.Cmd, sendChannel chan []byte, messageSouce PubSubSource, messageStore store.MessageStore) (*Receiver, error) {
	var err error
	rec := &Receiver{
		applicationId:       applicationId,
		sendChannel:         sendChannel,
		messageSouce:        messageSouce,
		messageStore:        messageStore,
		cancelChannel:       make(chan bool, 1),
		enableNotifications: true,
	}
	if len(cmd.Arg) == 0 || cmd.Arg[0] != '/' {
		return nil, fmt.Errorf("command requires at least a path argument, but non given")
	}
	args := strings.SplitN(cmd.Arg, " ", 3)
	rec.path = guble.Path(args[0])

	if len(args) > 1 {
		rec.doFetch = true
		rec.startId, err = strconv.ParseInt(args[1], 10, 64)
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

// start the receiver loop
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
				guble.Err("error while fetching: %v, %+v", err.Error(), rec)
				rec.sendError(guble.ERROR_INTERNAL_SERVER, err.Error())
				return
			}

			if err := rec.messageStore.DoInTx(rec.path.Partition(), rec.subscribeIfNoUnreadMessagesAvailable); err != nil {
				if err == unread_messages_available {
					//fmt.Printf(" unread_messages_available lastSendId=%v\n", rec.lastSendId)
					rec.startId = int64(rec.lastSendId + 1)
					continue // fetch again
				} else {
					guble.Err("error while subscribeIfNoUnreadMessagesAvailable: %v, %+v", err.Error(), rec)
					rec.sendError(guble.ERROR_INTERNAL_SERVER, err.Error())
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
			// the router kicked us out, because we are to slow for realtime listening,
			// so we setup parameters for fetching and cloging the gap. Than we can subscribe again.
			rec.startId = int64(rec.lastSendId + 1)
			rec.doFetch = true
		}
	}
}

func (rec *Receiver) subscribeIfNoUnreadMessagesAvailable(maxMessageId uint64) error {
	if maxMessageId > rec.lastSendId {
		return unread_messages_available
	}
	rec.subscribe()
	return nil
}

func (rec *Receiver) subscribe() {
	rec.route = NewRoute(string(rec.path), make(chan MsgAndRoute, 3), rec.applicationId, "TODO: remove userId from route")
	rec.messageSouce.Subscribe(rec.route)
	rec.sendOK(guble.SUCCESS_SUBSCRIBED_TO, string(rec.path))
}

func (rec *Receiver) receiveFromSubscription() {
	for {
		select {
		case msgAndRoute, ok := <-rec.route.C:
			if !ok {
				guble.Debug("messageSouce closed the channel returning from subscription", rec.applicationId)
				return
			}
			if guble.DebugEnabled() {
				guble.Debug("deliver message to applicationId=%v: %v", rec.applicationId, msgAndRoute.Message.MetadataLine())
			}
			if msgAndRoute.Message.Id > rec.lastSendId {
				rec.lastSendId = msgAndRoute.Message.Id
				rec.sendChannel <- msgAndRoute.Message.Bytes()
			} else {
				guble.Debug("dropping message %v, because it was already sent to client", msgAndRoute.Message.Id)
			}
		case <-rec.cancelChannel:
			rec.shouldStop = true
			rec.messageSouce.Unsubscribe(rec.route)
			rec.route = nil
			rec.sendOK(guble.SUCCESS_CANCELED, string(rec.path))
			return
		}

	}
}

func (rec *Receiver) fetchOnlyLoop() {
	err := rec.fetch()
	if err != nil {
		guble.Err("error while fetching: %v, %+v", err.Error(), rec)
		rec.sendError(guble.ERROR_INTERNAL_SERVER, err.Error())
	}
}

func (rec *Receiver) fetch() error {
	var err error

	fetch := store.FetchRequest{
		Partition:     rec.path.Partition(),
		MessageC:      make(chan store.MessageAndId, 3),
		ErrorCallback: make(chan error),
		StartCallback: make(chan int),
		Prefix:        []byte(rec.path),
		Count:         rec.maxCount,
	}

	if rec.startId >= 0 {
		fetch.Direction = 1
		fetch.StartId = uint64(rec.startId)
		if rec.maxCount == 0 {
			fetch.Count = math.MaxInt32
		}
	} else {
		fetch.Direction = -1
		if fetch.StartId, err = rec.messageStore.MaxMessageId(rec.path.Partition()); err != nil {
			return err
		}
		if rec.maxCount == 0 {
			fetch.Count = -1 * int(rec.startId)
		}
	}

	rec.messageStore.Fetch(fetch)

	for {
		select {
		case numberOfResults := <-fetch.StartCallback:
			rec.sendOK(guble.SUCCESS_FETCH_START, fmt.Sprintf("%v %v", rec.path, numberOfResults))
		case msgAndId, open := <-fetch.MessageC:
			if !open {
				rec.sendOK(guble.SUCCESS_FETCH_END, string(rec.path))
				return nil
			}
			guble.Debug("replay send %v, %v", msgAndId.Id, string(msgAndId.Message))
			rec.lastSendId = msgAndId.Id
			rec.sendChannel <- msgAndId.Message
		case err := <-fetch.ErrorCallback:
			return err
		case <-rec.cancelChannel:
			rec.shouldStop = true
			rec.sendOK(guble.SUCCESS_CANCELED, string(rec.path))
			// TODO implement cancellation in message store
			return nil
		}
	}
}

// stop/cancel the receiver
func (rec *Receiver) Stop() error {
	rec.cancelChannel <- true
	return nil
}

func (rec *Receiver) sendError(name string, argPattern string, params ...interface{}) {
	n := &guble.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: true,
	}
	rec.sendChannel <- n.Bytes()
}

func (rec *Receiver) sendOK(name string, argPattern string, params ...interface{}) {
	if rec.enableNotifications {
		n := &guble.NotificationMessage{
			Name:    name,
			Arg:     fmt.Sprintf(argPattern, params...),
			IsError: false,
		}
		rec.sendChannel <- n.Bytes()
	}
}
