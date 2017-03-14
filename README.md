# Gobbler Messaging Server

Gobbler is a simple user-facing messaging and data replication server written in Go.

[![Release](https://img.shields.io/github/release/cosminrentea/gobbler.svg)](https://github.com/cosminrentea/gobbler/releases/latest)
[![Docker](https://img.shields.io/docker/pulls/cosminrentea/gobbler.svg)](https://hub.docker.com/r/cosminrentea/gobbler/)
[![Build Status](https://api.travis-ci.org/cosminrentea/gobbler.svg?branch=master)](https://travis-ci.org/cosminrentea/gobbler)
[![Coverage Status](https://coveralls.io/repos/cosminrentea/gobbler/badge.svg?branch=master&service=github)](https://coveralls.io/github/cosminrentea/gobbler?branch=master)
[![GoDoc](https://godoc.org/github.com/cosminrentea/gobbler?status.svg)](https://godoc.org/github.com/cosminrentea/gobbler)
[![Go Report Card](https://goreportcard.com/badge/github.com/cosminrentea/gobbler)](https://goreportcard.com/report/github.com/cosminrentea/gobbler)
[![codebeat](https://codebeat.co/badges/363f61b0-caf3-440d-bd55-af92bdca42e1)](https://codebeat.co/projects/github-com-cosminrentea-gobbler-master)
[![Codacy](https://api.codacy.com/project/badge/Grade/91fa286a14ec460eb7f1fbb0d02e7888)](https://www.codacy.com/app/cosminrentea/gobbler?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=cosminrentea/gobbler&amp;utm_campaign=Badge_Grade)
[![Awesome-Go](https://camo.githubusercontent.com/13c4e50d88df7178ae1882a203ed57b641674f94/68747470733a2f2f63646e2e7261776769742e636f6d2f73696e647265736f726875732f617765736f6d652f643733303566333864323966656437386661383536353265336136336531353464643865383832392f6d656469612f62616467652e737667)](https://awesome-go.com)

# Overview
Gobbler is in an early state (release 0.4). 
It is already working well and is very useful, but the protocol, API and storage formats 
may still change (until reaching 0.7). 
If you intend to use Gobbler, please get in contact with us.

The goal of Gobbler is to be a simple and fast message bus for user interaction and replication of data between multiple devices:
* Very easy consumption of messages with web and mobile clients
* Fast realtime messaging, as well as playback of messages from a persistent commit log
* Reliable and scalable over multiple nodes
* User-aware semantics to easily support messaging scenarios between people using multiple devices
* Batteries included: usable as front-facing server, without the need of a proxy layer
* Self-contained: no mandatory dependencies to other services

## Working Features (0.4)

* Publishing and subscription of messages to topics and subtopics
* Persistent message store with transparent live and offline fetching
* WebSocket and REST APIs for message publishing
* Commandline client and Go client library
* Firebase Cloud Messaging (FCM) adapter: delivery of messages as FCM push notifications
* Docker images for server and client
* Simple Authentication and Access-Management
* Clean shutdown
* Improved logging using [logrus](https://github.com/Sirupsen/logrus) and logstash formatter
* Health-Check with Endpoint
* Collection of Basic Metrics, with Endpoint
* Added Postgresql as KV Backend
* Load testing with 5000 messages per instance
* Support for Apple Push Notification services (a new connector alongside Firebase)
* Upgrade, cleanup, abstraction, documentation, and test coverage of the Firebase connector
* GET list of subscribers / list of topics per subscriber (userID , deviceID) 
* Support for SMS-sending using Nexmo (a new connector alongside Firebase)

## Throughput
Measured on an old notebook with i5-2520M, dual core and SSD. Message payload was 'Hello Word'.
Load driver and server were set up on the same machine, so 50% of the cpu was allocated to the load driver.

* End-2-End: Delivery of ~35.000 persistent messages per second
* Fetching: Receive of ~70.000 persistent messages per second

During the tests, the memory consumption of the server was around ~25 MB.

## Table of Contents

- [Roadmap](#roadmap)
  - [Roadmap Release 0.5](#roadmap-release-05)
  - [Roadmap Release 0.6](#roadmap-release-06)
  - [Roadmap Release 0.7](#roadmap-release-07)
- [Gobbler Docker Image](#gobbler-docker-image)
  - [Start the Gobbler Server](#start-the-gobbler-server)
  - [Connecting with the Gobbler Client](#connecting-with-the-gobbler-client)
- [Build and Run](#build-and-run)
  - [Build and Start the Server](#build-and-start-the-server)
    - [Configuration](#configuration)
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

# Roadmap
This is the current (and fast changing) roadmap and todo list:

## Roadmap Release 0.5
* Replication across multiple servers (in a Gobbler cluster)
* Acknowledgement of message delivery for connectors
* Storing the sequence-Id of topics in KV store, if we turn off persistence
* Filtering of messages in gobbler server (e.g. sent by the REST client) according to URL parameters: UserID, DeviceID, Connector name
* Updating README to show subscribe/unsubscribe/get/posting, health/metrics 

## Roadmap Release 0.6
* Make notification messages optional by client configuration
* Correct behaviour of receive command with `maxCount` on subtopics
* Cancel of fetch in the message store and multiple concurrent fetch commands for the same topic
* Configuration of different persistence strategies for topics
* Delivery semantics: user must read on one device / deliver only to one device / notify if not connected, etc.
* User-specific persistent subscriptions across all clients of the user
* Client: (re-)setup of subscriptions after client reconnect
* Message size limit configurable by the client with fetching by URL

## Roadmap Release 0.7
* HTTPS support in the service
* Minimal example: chat application
* (TBD) Improved authentication and access-management
* (TBD) Add Consul as KV Backend
* (TBD) Index-based search of messages using [GoLucene](https://github.com/balzaczyy/golucene)

# Gobbler Docker Image
We are providing Docker images of the server and client for your convenience.

## Start the Gobbler Server
There is an automated Docker build for the master at the Docker Hub.
To start the server with Docker simply type:
```
docker run -p 8080:8080 cosminrentea/gobbler
```

To see available configuration options:
```
docker run cosminrentea/gobbler --help
```

All options can be supplied on the commandline or by a corresponding environment variable with the prefix `GUBLE_`.
So to let gobbler be more verbose, you can either use:
```
docker run cosminrentea/gobbler --log=info
```
or
```
docker run -e GUBLE_LOG=info cosminrentea/gobbler
```

The Docker image has a volume mount point at `/var/lib/gobbler`, so if you want to bind-mount the persistent storage from your host you should use:
```
docker run -p 8080:8080 -v /host/storage/path:/var/lib/gobbler cosminrentea/gobbler
```

## Connecting with the Gobbler Client
The Docker image includes the gobbler commandline client `gobbler-cli`.
You can execute it within a running gobbler container and connect to the server:
```
docker run -d --name gobbler cosminrentea/gobbler
docker exec -it gobbler /usr/local/bin/gobbler-cli
```
Visit the [`gobbler-cli` documentation](https://github.com/cosminrentea/gobbler/tree/master/gobbler-cli) for more details.

# Build and Run
Since Go makes it very easy to build from source, you can compile gobbler using a single command.
A prerequisite is having an installed Go environment and an empty directory:
```
sudo apt-get install golang
mkdir gobbler && cd gobbler
export GOPATH=`pwd`
```

## Build and Start the Server
Build and start gobbler with the following commands (assuming that directory `/var/lib/gobbler` is already created with read-write rights for the current user):
```
go get github.com/cosminrentea/gobbler
bin/gobbler --log=info
```

### Configuration

|CLI Option|Env Variable|Values|Default|Description|
|--- |--- |--- |--- |--- |--- |
|--env|GUBLE_ENV|development &#124; integration &#124; preproduction &#124; production|development|Name of the environment on which the application is running. Used mainly for logging|
|--health-endpoint|GUBLE_HEALTH_ENDPOINT|resource/path/to/healthendpoint|/admin/healthcheck|The health endpoint to be used by the HTTP server.Can be disabled by setting the value to ""|
|--http|GUBLE_HTTP_LISTEN|format: [host]:port||The address to for the HTTP server to listen on|
|--kvs|GUBLE_KVS|memory &#124; file &#124; postgres|file|The storage backend for the key-value store to use|
|--log|GUBLE_LOG|panic &#124; fatal &#124; error &#124; warn &#124; info &#124; debug|error|The log level in which the process logs|
|--metrics-endpoint|GUBLE_METRICS_ENDPOINT|resource/path/to/metricsendpoint|/admin/metrics|The metrics endpoint to be used by the HTTP server.Can be disabled by setting the value to ""|
|--ms|GUBLE_MS|memory &#124; file|file|The message storage backend|
|--profile|GUBLE_PROFILE|cpu &#124; mem &#124; block||The profiler to be used|
|--storage-path|GUBLE_STORAGE_PATH|path/to/storage|/var/lib/gobbler|The path for storing messages and key-value data like subscriptions if defined.The path must exists!|


#### APNS

|CLI Option|Env Variable|Values|Default|Description|
|--- |--- |--- |--- |--- |--- |
|--apns|GUBLE_APNS|true &#124; false|false|Enable the APNS module in general as well as the connector to the development endpoint|
|--apns-production|GUBLE_APNS_PRODUCTION|true &#124; false|false|Enables the connector to the apns production endpoint, requires the apns option to be set|
|--apns-cert-file|GUBLE_APNS_CERT_FILE|path/to/cert/file||The APNS certificate file name, use this as an alternative to the certificate bytes option|
|--apns-cert-bytes|GUBLE_APNS_CERT_BYTES|cert-bytes-as-hex-string||The APNS certificate bytes, use this as an alternative to the certificate file option|
|--apns-cert-password|GUBLE_APNS_CERT_PASSWORD|password||The APNS certificate password|
|--apns-app-topic|GUBLE_APNS_APP_TOPIC|topic||The APNS topic (as used by the mobile application)|
|--apns-prefix|GUBLE_APNS_PREFIX|prefix|/apns/|The APNS prefix / endpoint|
|--apns-workers|GUBLE_APNS_WORKERS|number of workers|Number of CPUs|The number of workers handling traffic with APNS (default: number of CPUs)|


#### SMS

|CLI Option|Env Variable|Values|Default |Description|
|--- |--- |--- |--- |--- |--- |
|sms|GUBLE_SMS|true &#124; false|false |Enable the SMS gateway|
|sms_api_key|GUBLE_SMS_API_KEY|api key||The Nexmo API Key for Sending sms|
|sms_api_secret|GUBLE_SMS_API_SECRET|api secret||The Nexmo API Secret for Sending sms|
|sms_topic|GUBLE_SMS_TOPIC|topic|/sms|The topic for sms route|
|sms_workers|GUBLE_SMS_WORKERS|number of workers|Number of CPUs|The number of workers handling traffic with Nexmo sms endpoint|

#### FCM

|CLI Option|Env Variable|Values|Default|Description|
|--- |--- |--- |--- |--- |--- |
|--fcm|GUBLE_FCM|true &#124; false|false|Enable the Google Firebase Cloud Messaging connector|
|--fcm-api-key|GUBLE_FCM_API_KEY|api key||The Google API Key for Google Firebase Cloud Messaging|
|--fcm-workers|GUBLE_FCM_WORKERS|number of workers|Number of CPUs|The number of workers handling traffic with Firebase Cloud Messaging|
|--fcm-endpoint|GUBLE_FCM_ENDPOINT|format: url-schema|https://fcm.googleapis.com/fcm/send|The Google Firebase Cloud Messaging endpoint|
|--fcm-prefix|GUBLE_FCM_PREFIX|prefix|/fcm/|The FCM prefix / endpoint|

#### Postgres

|CLI Option|Env Variable|Values|Default|Description|
|--- |--- |--- |--- |--- |--- |
|--pg-host|GUBLE_PG_HOST|hostname|localhost|The PostgreSQL hostname|
|--pg-port|GUBLE_PG_PORT|port|5432|The PostgreSQL port|
|--pg-user|GUBLE_PG_USER|user|gobbler|The PostgreSQL user|
|--pg-password|GUBLE_PG_PASSWORD|password|gobbler|The PostgreSQL password|
|--pg-dbname|GUBLE_PG_DBNAME|database|gobbler|The PostgreSQL database name|


## Run All Tests
```
go get -t github.com/cosminrentea/gobbler/...
go test github.com/cosminrentea/gobbler/...
```

# Clients
The following clients are available:
* __Commandline Client__: https://github.com/cosminrentea/gobbler/tree/master/gobbler-cli
* __Go client library__: https://github.com/cosminrentea/gobbler/tree/master/client
* __JavaScript library__: (in early stage) https://github.com/cosminrentea/gobbler-js

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
You can set fields in the header JSON of the message by providing the corresponding HTTP headers with the prefix `X-Gobbler-`.

Curl example with the resulting message:
```
curl -X POST -H "x-Gobbler-Key: Value" --data Hello 'http://127.0.0.1:8080/api/message/foo?userId=marvin&messageId=42'
```
Results in:
```
16,/foo,marvin,VoAdxGO3DBEn8vv8,42,1451236804
{"Key":"Value"}
Hello
```

## WebSocket Protocol
The communication with the gobbler server is done by ordinary WebSockets, using a binary encoding.

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

#### Subscribe/Receive
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
  because your client is too slow to read all incoming messages.

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

#### Unsubscribe Success Notification
An unsubscribe/cancel operation is confirmed by the following notification:
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
The server takes care, that a message only gets delivered once, even if it is matched by multiple
subscription paths.

### Subtopics
The path delimiter gives the semantic of subtopics. 
With this, a subscription to a parent topic (e.g. `/foo`)
also results in receiving all messages of the subtopics (e.g. `/foo/bar`).
