# guble messaging server

guble is a simple messaging server, written in golang.
Guble is in a very early state and unreleased. If you like it,
help me or come back later ;-)

## Roadmap

If there is enough time, the following features may be realized:

* Subscription to multiple topics
* Replay of messages
* Additional REST API for message Publishing
* Authentication and Accessmanagement

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
Messages send from the server to the client are all of in the following form:
```
    <id>,<fromUserId>,<fromClientId>,<path>,<mime-type>:<body>
    e.g.
    42,user01,phone1,/foo/bar,text/plain:Hello World
    42,user01,54sdcj8sd7,/foo/bar,binary:anyByteData
```

* All text formats are assumed as utf-8 encoded.
* Message ids are int64, and distinct within a topic. The message ids are strictly monotonically increasing
  depending on the message age, but there is no guarantee for a correct order while transmitting.

### Client Commands
The client can send the following commands:
```
    // Send a message to a topic
    send <path> <body>
    e.g.
    send /foo Hello World
```

```
    // Subscribe to a path (e.g. a topic or subtopic)
    subscribe <path>
    e.g.
    subscribe /foo
    subscribe /foo/bar
```

```
    // Planned: Unsubscribe from a path (e.g. a topic or subtopic)
    unsubscribe <path>
    e.g.
    unsubscribe /foo
    unsubscribe /foo/bar
```

```
    // Planned: Replay all messages from a specific topic, which are newer than the supllied message id.
    replay <lastMessageId> /<topic>
    e.g.
    replay 42 /events
```
