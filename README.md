# guble messaging server

guble is a simple user facing messaging and data replication server, written in golang.

[![Build Status](https://api.travis-ci.org/smancke/guble.svg)](https://travis-ci.org/smancke/guble)

# Overview
Guble is in an early state and unreleased. The implemented features are already useful and working well,
but we may change api's and implementation details, until reaching v0.5.

The goal of guble is to be a simple and fast message bus for user interaction and replication of data between multiple devices:
* Very easy consumption of messages with web and mobile clients.
* Fast realtime messaging, as well as playback of messages from a persistant commit-log.
* Reliable and scalable over multiple nodes.
* User aware sematics, to easily support message szenarios between people with their multiple devices.
* Batteries included: Guble should be usable as front facing server, without the need of a proxy layer.
* Self contained: No mandatory dependencies to other services.

## Working Features

* In-memory dispatching of messages
* Persistance of topics with replay of messages
* Websocket api
* Commandline client and go client library
* Google cloud messaging adapter: Delivery of messages as gcm push notifications
* Subscription to multiple topics and subtopics
* Throughput: Delivery of ~50.000 messages per second (end-to-end)
* Docker image for client and server

## Table of Contents

- [Roadmap](#roadmap)
  - [Release 0.1](#release-01)
  - [Roadmap Release 0.2](#roadmap-release-02)
  - [Roadmap Release 0.3](#roadmap-release-03)
  - [Roadmap Release 0.4](#roadmap-release-04)
- [Guble docker image](#guble-docker-image)
- [Build and run](#build-and-run)
  - [Build and start the sever](#build-and-start-the-sever)
  - [Run all tests](#run-all-tests)
- [Clients](#clients)
- [Protocol-Reference](#protocol-reference)
  - [Rest Api](#rest-api)
  - [WebsocketProtocol](#websocket-protocol)
    - [Message-Format](#message-format)
    - [Client Commands](#client-commands)
  - [Topics](#topics)
  - [Authentication and Accessmanagement](#authentication-and-accessmanagement)

# Roadmap
This is the current (and fast changing) roadmap and todo-list:

## Release 0.1
The first release 0.1 is expected start of January 2016
* Persistance and replay of messages
** Replay command implementation
** Replay command in cli help
* Better Approach for message buffering on huge message numbers
* Clean Shutdown

## Roadmap Release 0.2
* Improve Logging (Maybe use of: https://github.com/Sirupsen/logrus)
* Rename package guble to protocol or gublep
* Change time from iso to timestamp
* Client: (Re)-Setup of subscriptions after client reconnect
* Stable Java-Script Client: https://github.com/smancke/guble-js
* Correct behaviour for receive command with maxCount on subtopics
* Make Notification Messages optional by client configuration
* Cancel of fetch in the message store and multiple concurrent fetch Commands for the same topic

## Roadmap Release 0.3
* Cleanup, documentation, and test coverage of the gcm connector
* Authentication and Access Management
* Configuration of a cross origin policy
* Configuration of different persistant strategies for topics
* Client configurable message size limit with fetching by url

## Roadmap Release 0.4
* Replication across multiple Servers
* Delivery semantics (e.g. user must read on one device, deliver only to one device, notify if not connected, ..)
* Maybe: Acknowledgement of message delivery

# Guble docker image
## Start the guble server
There is an automated docker build for the master at docker hub.
To start the server with docker simple type:
```
	docker run -p 8080:8080 smancke/guble
```

See available configuration options:
```
	docker run smancke/guble --help
```

All options can be supplied by command line or by a corresponding environment variable with the prefix `GUBLE_'.
So, to let guble be more verbose, you can either use:
```
	docker run smancke/guble --log-info
```
or
```
	docker run -e GUBLE_LOG_INFO=true smancke/guble
```

## Connecting with the guble-cli
The docker image has the guble command line client included. You can execute it within a running golang container and
connect to the server.
```
docker run -d --name guble smancke/guble
docker exec -it guble /go/bin/guble-cli
```
Further documentation of the guble-cli can be found here: https://github.com/smancke/guble/tree/master/guble-cli

# Build and run
Since go makes it very easy to build from source, you can
compile guble with one command line.
Prerequirement is an installed go environment and an empty directory. e.g.
```
sudo apt-get install golang
mkdir guble && cd guble
export GOPATH=`pwd`
```

## Build and start the sever
Starting the guble server
Build and start with the following commands:
```
	go get github.com/smancke/guble
	bin/guble --log-info
```

## Run all tests
```
    go get -t github.com/smancke/guble/...
    go test github.com/smancke/guble/...
```

# Clients
The following clients are available
* __Commandline Client__: https://github.com/smancke/guble/tree/master/guble-cli
* __Go client library__: https://github.com/smancke/guble/tree/master/client
* __Java Script library__: (in beginning) https://github.com/smancke/guble-js

# Protocol Reference

## Rest Api
Currently there is a minimalistic rest api, just for publishing messages.

```
POST /api/message/<topic>
```
Url Prameters:
* __userId__: The PublisherUserId
* __messageId__: The PublisherMessageId

Headers:
You can set fields in the header json from the message by providing the corresponding http headers with the prefix `X-Guble-`.

Curl Example with the resulting message:
```
curl -X POST -H "x-Guble-Key: Value" --data Hello 'http://127.0.0.1:8080/api/message/foo?userId=marvin&messageId=42'

# leads to:
16,/foo,marvin,VoAdxGO3DBEn8vv8,42,2015-12-27T18:20:04+01:00
{"Key":"Value"}
Hello
```

## Websocket Protocol
The communication with the guble server is done by usual websockets, using the binary encoding.

### Message Format
Payload messages send from the server to the client are all of in the following form:
```
    <path:string>,<sequenceId:int64>,<publisherUserId:string>,<publisherApplicationId:string>,<publisherMessageId:string>,<messagePublishingTime:iso-date>\n
    [<application headers json>]\n
    <body>

    example 1:
    /foo/bar,42,user01,phone1,id123,2015-01-01T12:00:00+01:00
    {"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
    Hello World

    example 2:
    /foo/bar,42,user01,54sdcj8sd7,id123,2015-01-01T12:00:00+01:00

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

#### Receive
Receive messages from a path (e.g. a topic or subtopic).
This command can be used to subscribe for incoming messages on a topic,
as well as for replaying the message history.
```
    + <path> [<startId>[,<maxCount>]]
```
* `path`: The topic to receive the messages from.
* `startId`: The message id to start the replay.
** If no startId is given, only future messages will be received (simple subscribe)
** If the startId is negative, it is interpreted as relative count of last messages in the history.
* `maxCount`: The maximum number of messages to replay.

__Note__: Currently the maxCount does not work correctly on subtopics.

Examples:
```
    + /foo         # Subscribe to all future messages matching /foo
    + /foo/bar     # Subscribe to all future messages matching /foo/bar

    + /foo 0       # Receive all message from the topic and subscribe for further incoming messages.

    + /foo 42      # Receive all message with message ids >= 42
                   # from the topic and subscribe for further incoming messages.

    + /foo 0 20    # Receive the first (oldest) 20 messages within the topic and stop.
                   # (If the topic has less messages, it will stop after receiving all existing ones.)

    + /foo -20     # Receive the last (newest) 20 messages from the topic and then
                   # subscribe for further incoming messages.

    + /foo -20 20  # Receive the last (newest) 20 messages within the topic and stop.
                   # (If the topic has less messages, it will stop after receiving all existing ones.)
```

#### Unsubscribe/Cancel
Cancel further receiving of messages from a path (e.g. a topic or subtopic).

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

### User Topics `/user`
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
