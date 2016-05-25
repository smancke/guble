# Guble Messaging Server

Guble is a simple user-facing messaging and data replication server written in Go.

[![Release](https://img.shields.io/github/release/smancke/guble.svg)](https://github.com/smancke/guble/releases/latest) [![Docker](https://img.shields.io/docker/pulls/smancke/guble.svg)](https://hub.docker.com/r/smancke/guble/) [![Build Status](https://api.travis-ci.org/smancke/guble.svg)](https://travis-ci.org/smancke/guble) [![Coverage Status](https://coveralls.io/repos/smancke/guble/badge.svg?branch=master&service=github)](https://coveralls.io/github/smancke/guble?branch=master) [![Go Report Card](https://goreportcard.com/badge/github.com/smancke/guble)](https://goreportcard.com/report/github.com/smancke/guble)

# Overview
Guble is in an early state. It is already working well and is very useful, but the protocol, API and storage formats may
change without further announcement (until reaching 0.5). If you intend to already use guble, please get in contact with us.

The goal of guble is to be a simple and fast message bus for user interaction and replication of data between multiple devices:
* Very easy consumption of messages with web and mobile clients
* Fast realtime messaging, as well as playback of messages from a persistent commit log
* Reliable and scalable over multiple nodes
* User-aware semantics to easily support messaging scenarios between people using multiple devices
* Batteries included: usable as front-facing server, without the need of a proxy layer
* Self-contained: no mandatory dependencies to other services

## Working Features (0.1)

* Publishing and subscription of messages to topics and subtopics
* Persistent message store with transparent live and offline fetching
* WebSocket and REST APIs for message publishing
* Commandline client and Go client library
* Google Cloud Messaging adapter: delivery of messages as GCM push notifications
* Docker image for client and server

## Throughput
Measured on an old notebook with i5-2520M, dual core and SSD. Payload was 'Hello Word'.
Load driver and server were set up on the same machine, so 50% of the cpu was allocated to the load driver.

* End-2-End: Delivery of ~35.000 persistent messages per second
* Fetching: Receive of ~70.000 persistent messages per second

During the tests, the memory consumption of the server was around ~25 MB.

## Table of Contents

- [Roadmap](#roadmap)
  - [Roadmap Release 0.2](#roadmap-release-02)
  - [Roadmap Release 0.3](#roadmap-release-03)
  - [Roadmap Release 0.4](#roadmap-release-04)
- [Guble Docker Image](#guble-docker-image)
  - [Start the Guble Server](#start-the-guble-server)
  - [Connecting with the Guble Client](#connecting-with-the-guble-client)
- [Build and Run](#build-and-run)
  - [Build and Start the Server](#build-and-start-the-server)
  - [Run All Tests](#run-all-tests)
- [Clients](#clients)
- [Protocol Reference](#protocol-reference)
  - [REST API](#rest-api)
    - [Headers](#headers)
  - [WebSocket Protocol](#websocket-protocol)
    - [Message Format](#message-format)
    - [Client Commands](#client-commands)
    - [Server Status Messages](#server-status-messages)
  - [Topics](#topics)
    - [Subtopics](#subtopics)
    - [User Topics](#user-topics)
    - [Group Topics](#group-topics)

# Roadmap
This is the current (and fast changing) roadmap and todo list:

## Roadmap Release 0.2
This release contains a lot of small changes and the JavaScript API.

* Authentication and access management
* Clean shutdown
* Improve logging (Maybe use of https://github.com/Sirupsen/logrus)
* Rename package "guble" to "protocol" or "gublep"
* ~~Change time from ISO 8601 to unix timestamp~~
* Remove `userId` from route

## Roadmap Release 0.3
* Stable JavaScript client: https://github.com/smancke/guble-js
* Add Consul as KV Backend strategy
* Storing the sequence Id of topics in kv store, if we turn of persistance
* Replication across multiple servers
* Cleanup, documentation, and test coverage of the GCM connector
* Make notification messages optional by client configuration
* Load testing with 5000 connections per instance
* Introduce Health Check Ressource
* Introduce Basic Metrics API

## Roadmap Release 0.4
* Change sqlite to bolt
* (TBD) Index-based search of messages using [GoLucene](https://github.com/balzaczyy/golucene)
* (TBD) Acknowledgement of message delivery
* Correct behaviour of receive command with `maxCount` on subtopics
* Configuration of different persistence strategies for topics

## Roadmap Release 0.5
* Delivery semantics: user must read on one device, deliver only to one device, notify if not connected, etc.
* HTTPS support in the service
* Cancel of fetch in the message store and multiple concurrent fetch commands for the same topic
* Minimal example chat application
* Userspecific persistant subscriptions accross all clients of the user
* Client: (re-)setup of subscriptions after client reconnect
* Message size limit configurable by the client with fetching by URL

# Guble Docker Image
We are providing Docker images of the server and client for your convenience.

## Start the Guble Server
There is an automated Docker build for the master at the Docker Hub.
To start the server with Docker simply type:
```
docker run -p 8080:8080 smancke/guble
```

To see available configuration options:
```
docker run smancke/guble --help
```

All options can be supplied on the commandline or by a corresponding environment variable with the prefix `GUBLE_`.
So to let guble be more verbose, you can either use:
```
docker run smancke/guble --log-info
```
or
```
docker run -e GUBLE_LOG_INFO=true smancke/guble
```

The Docker image has a volume mount point at `/var/lib/guble`. So you should use
```
docker run -p 8080:8080 -v /host/storage/path:/var/lib/guble smancke/guble
```
if you want to bind-mount the persistent storage from your host.

## Connecting with the Guble Client
The Docker image includes the guble commandline client `guble-cli`.
You can execute it within a running guble container and connect to the server:
```
docker run -d --name guble smancke/guble
docker exec -it guble /go/bin/guble-cli
```
Visit the [`guble-cli` documentation](https://github.com/smancke/guble/tree/master/guble-cli) for more details.

# Build and Run
Since Go makes it very easy to build from source, you can compile guble using a single command.
A requisite is having an installed Go environment and an empty directory:
```
sudo apt-get install golang
mkdir guble && cd guble
export GOPATH=`pwd`
```

## Build and Start the Server
Build and start guble with the following commands:
```
go get github.com/smancke/guble
bin/guble --log-info
```

## Run All Tests
```
go get -t github.com/smancke/guble/...
go test github.com/smancke/guble/...
```

# Clients
The following clients are available:
* __Commandline Client__: https://github.com/smancke/guble/tree/master/guble-cli
* __Go client library__: https://github.com/smancke/guble/tree/master/client
* __JavaScript library__: (in early stage) https://github.com/smancke/guble-js

# Protocol Reference

## REST API
Currently there is a minimalistic REST API, just for publishing messages.

```
POST /api/message/<topic>
```
URL parameters:
* __userId__: The PublisherUserId
* __messageId__: The PublisherMessageId

### Headers
You can set fields in the header JSON of the message by providing the corresponding HTTP headers with the prefix `X-Guble-`.

Curl example with the resulting message:
```
curl -X POST -H "x-Guble-Key: Value" --data Hello 'http://127.0.0.1:8080/api/message/foo?userId=marvin&messageId=42'
```
Results in:
```
16,/foo,marvin,VoAdxGO3DBEn8vv8,42,1451236804
{"Key":"Value"}
Hello
```

## WebSocket Protocol
The communication with the guble server is done by ordinary WebSockets, using a binary encoding.

### Message Format
All payload messages sent from the server to the client are using the following format:
```
<path:string>,<sequenceId:int64>,<publisherUserId:string>,<publisherApplicationId:string>,<publisherMessageId:string>,<messagePublishingTime:unix-timestamp>\n
[<application headers json>]\n
<body>

example 1:
/foo/bar,42,user01,phone1,id123,1420110000
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World

example 2:
/foo/bar,42,user01,54sdcj8sd7,id123,1420110000

anyByteData
```

* All text formats are assumed to be UTF-8 encoded.
* Message `sequenceId`s are `int64`, and distinct within a topic.
  The message `sequenceId`s are strictly monotonically increasing depending on the message age, but there is no guarantee for the right order while transmitting.

### Client Commands
The client can send the following commands.

#### Send
Publish a message to a topic:
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
* `path`: the topic to receive the messages from
* `startId`: the message id to start the replay
** If no `startId` is given, only future messages will be received (simple subscribe).
** If the `startId` is negative, it is interpreted as relative count of last messages in the history.
* `maxCount`: the maximum number of messages to replay

__Note__: Currently, the fetching of stored messages does not recognize subtopics.

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

### Server Status Messages
The server sends status messages to the client. All positive status messages start with `>`.
Status messages reporting an error start with `!`. Status messages are in the following format.

```
'#'<msgType> <Explanation text>\n
<json data>
```

#### Connection Message
```
#ok-connected You are connected to the server.\n
{"ApplicationId": "the app id", "UserId": "the user id", "Time": "the server time as unix timestamp "}
```

Example:
```
#connected You are connected to the server.
{"ApplicationId": "phone1", "UserId": "user01", "Time": "1420110000"}
```

#### Send Success Notification
This notification confirms, that the messaging system has successfully received the message and now starts transmitting it to the subscribers:

```
#send <publisherMessageId>
{"sequenceId": "sequence id", "path": "/foo", "publisherMessageId": "publishers message id", "messagePublishingTime": "unix-timestamp"}
```

#### Receive Success Notification
Depending on the type of `+` (receive) command, up to three different notification messages will be sent back.
Be aware, that a server may send more receive notifications that you would have expected in first place, e.g. when:
* Additional messages are stored, while the first fetching is in progress
* The server decides to meanwhile stop the online subscription and change to fetching,
  because your client is to slow to read all incoming messages.

1. When the fetch operation starts:

    ```
    #fetch-start <path> <count>
    ```
    * `path`: the topic path
    * `count`: the number of messages that will be returned

2. When the fetch operation is done:

    ```
    #fetch-done <path>
    ```
    * `path`: the topic path
3. When the subscription to new messages was taken:

    ```
    #subscribed-to <path>
    ```
    * `path`: the topic path

#### Cancel Success Notification
A cancel operation is confirmed by the following notification:
```
#canceled <path>
```

#### Send Error Notification
This message indicates, that the message could not be delivered.
```
!error-send <publisherMessageId> <error text>
{"sequenceId": "sequence id", "path": "/foo", "publisherMessageId": "publishers message id", "messagePublishingTime": "unix-timestamp"}
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

Messages can be hierarchically routed by topics, so they are represented by a path, separated by `/`.
There are two global topic namespaces: `/user` and `/group`.
The server takes care, that a message only gets delivered once, even if it is matched by multiple
subscription paths.

### Subtopics
The path delimiter gives the semantic of subtopics. With this, a subscription to a parent topic (e.g. `/foo`)
also results in receiving all message of the sub topics (e.g. `/foo/bar`).

### User Topics
Each user has its own topic namespace.
```
/user/<userId>
```
Within this namespace, every device or application the user is connected with can create its own topic:
```
/user/<userId>/<applicationId>
```
In addition to this, there is a topic for all devices:
```
/user/<userId>/common
```
As soon as the application is connected, it gets automatically subscribed to the topics `applicationId` and `common`.
So other applications can address this application by sending messages to one of these topics.
Applications are free to send messages to any subtopic within the user namespace.
Subtopics other than `applicationId` or `common` are also addressable, but not subscribed by default.
If one sends a message to `/user/<userId>/foo`, only those applications of the user will receive it, that have explicitly subscribed to it.

### Group Topics
(TODO, not implemented in the first version)

Multiple users can share a group where every member of the group can send to topics and subscribe to them.
The topics of a group are located under:
```
/user/<groupId>/
```
