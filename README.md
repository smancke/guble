# guble messaging server [![Build Status](https://api.travis-ci.org/smancke/guble.svg)](https://travis-ci.org/smancke/guble)

guble is a simple messaging server, written in golang.
Guble is in a very early state and unreleased. For now, this documentation is a draft, how it should work at a later state! If you like it, help me, or come back later ;-)

The goals of guble are, to have a message bus which is:
* Very easy consumption of messages with web and mobile clients
* Consumers should not need an api in addition to web sockets
* Fast realtime messaging, as well as playback of messages from a commit-log
* Reliable and scalable over multiple nodes
* User aware sematics, to easily support message szenarios between people with their multiple devices

## Features

* In-memory dispatching of messages
* Subscription to multiple topics and subtopics

## Next TODOs

* Connection message
* User Topics
* Rest Endpoint for message publishing
* Improve Logging
* Clean Shutdown
* Better Approach for message buffering on huge message numbers
* Client: (Re)-Setup of subscriptions after client reconnect

## Roadmap

If there is enough time, the following features may be realized:

* Persistance and replay of messages
* Acknowledgement of message delivery
* Delivery semantics (e.g. user must read on one device)
* Replication across multiple Servers
* Additional REST API for message Publishing
* Authentication and Accessmanagement
* CRC32 Checksum for messages

## Starting the guble server
Build and start with the following commands:
```
	go get -t github.com/smancke/guble
	./bin/guble
```

## The gubble CLI
For simle tesing and interaction, there is a cli client for gubble.
Build and start with the following commands:

```
	go install github.com/smancke/guble/guble-cli:
	./bin/guble-cli
```

## Protocol
The communication with the guble server is done by usual websockets.

### Message Format
Payload messages send from the server to the client are all of in the following form:
```
    <sequenceId:int64>,<path:string>,<publisherUserId:string>,<publisherApplicationId:string>,<publisherMessageId:string>,<messagePublishingTime:iso-date>\n
    [<application headers json>]\n
    <body>

    example 1:
    42,/foo/bar,user01,phone1,id123,2015-01-01T12:00:00+01:00
    {"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
    Hello World

    example 2:
    42,/foo/bar,user01,54sdcj8sd7,id123,2015-01-01T12:00:00+01:00

    anyByteData
```

* All text formats are assumed to be utf-8 encoded.
* Message sequenceId are int64, and distinct within a topic. The message sequenceIds are strictly monotonically increasing
  depending on the message age, but there is no guarantee for a correct order while transmitting.

### Client Commands
The client can send the following commands.


#### Send
Publish a message for a topic
```
    > <path> [<publisherMessageId>]\n
    [<header>\n]..
    \n
    <body>

    example:
    > /foo

    Hello World
```

#### Subscribe
Subscribe to a path (e.g. a topic or subtopic)
```
    + <path>

    example:
    + /foo
    + /foo/bar
```

#### Unsubscribe  (TBD)
Unsubscribe from a path (e.g. a topic or subtopic)
```
    - <path>

    example:
    - /foo
    - /foo/bar
```

#### Replay (TBD, not implemented in the first version)
Replay all messages from a specific topic, which are newer than the supllied message id.
If `maxCount` is supplied, only the maxCount newest messages are supplied.
```
    replay <lastSequenceId>[,<maxCount>] /<topic>

    examples:
    // replay all messages in the topic /events:
    replay -1 /events
    
    // replay all messages with sequenceId > 42:
    replay 42 /events
    
    // replay the last 10 messages:
    replay -1,10 /events
```

### Server Status messages
The server sends status messages to the client. All positive status messages start with `>`.
Status messages reporting an error start with `!`. Status messages are in the form.

```
    '#'<msgType> <Explanation text>\n
    <json data>

    example:

```

#### Connection message
```
    #ok-connected You are connected to the server.\n
    {"ApplicationId": "the app id", "UserId": "the user id", "Time": "the server time as iso date"}

    example:
    #ok-connected You are connected to the server.
    {"ApplicationId": "phone1", "UserId": "user01", "Time": "2015-01-01T12:00:00+01:00"}
```

#### Send success notification
This notification confirms, that the messaging system has successfully received the message and now starts transmiting it to the subscribers.

```
    #ok-send <publisherMessageId>
    {"sequenceId": "sequence id", "path": "/foo", "publisherMessageId": "publishers message id", "messagePublishingTime": "iso-date"}
```

#### Subscribe success notification
This notification confirms, a sucessful subscribe message.

```
    #ok-subscribed-to <path>
```

#### Send error notification
This message indicates, that the message could not be delivered.
```
    !error-send <publisherMessageId> <error text>
    {"sequenceId": "sequence id", "path": "/foo", "publisherMessageId": "publishers message id", "messagePublishingTime": "iso-date"}
```

#### Bad Request
This notification has the same meaning as the http 400 Bad Request.
```
    !error-bad-request unknown command 'sdcsd'
```

#### Internal Server Error
This notification has the same meaning as the http 500 Internal Server Error.
```
    !error-server-internal this computing node has problems
```

## Topics 

Messages can be routed by topics are hierarchically, so they are represented by a path, separated by `/`.
There are two global topic spaces `/user` and `/group`.
The server takes care, that a message only gets delivered once, even if it was matched by multiple
subscription paths.

### Subtopics
The path delimiter gives the semantic of subtopics. With this, a subscription to a parent topic (e.g. `/foo`)
also results in receiving all message of the sub topics (e.g. `/foo/bar`).

### User Topics `/user` (TBD, not implemented in the first version)
Each user has its own Topic space.
```
    /user/<userId>
```
Within this space, every device or application, the user is connected with, creates it's own topic:
```
    /user/<userId>/<applicationId>
```
In addition to this, there is a topic fo all devices:
```
    /user/<userId>/common
```
As soon, as the application is connected, it gets automatically subscribed to the `applicationId` topic and to
the `common` topic. So other applications can address this application by sending messages to one of both queues.
Applications are free to send messages to any subtopic within the user space.
Subtopics other than the `applicationId` or the `common` are also addressable, but not subscribed by default.
If one sends a message to `/user/<userId>/foo`, only those applications of the user will receive it, who have explicitly subscribed to it.

### Group Topics `/group` (TBD, not implemented in the first version)
Multiple users can share a group where every member of the group can send to topics and subscribe on them.
The topics of such a group are located at:
```
    /user/<groupId>
```

## Authentication and Accessmanagement
TBD ..
