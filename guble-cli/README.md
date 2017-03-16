# The guble command line client

This is the command line client for the guble messaging server. It is intended
for demonstration and debugging use.

[![Build Status](https://api.travis-ci.org/cosminrentea/gobbler.svg)](https://travis-ci.org/cosminrentea/gobbler)


## Starting the client with docker 
The guble docker image has the command line client included. You can execute it within a running golang container and
connect to the server.
```
docker run -d --name gobbler cosminrentea/gobbler
docker exec -it gobbler /go/bin/guble-cli
```


## Building from source
```
	go get github.com/cosminrentea/gobbler/guble-cli
	bin/guble-cli
```

## Start options
```
usage: guble-cli [--exit] [--verbose] [--url URL] [--user USER] [--log-info] [--log-debug] [COMMANDS [COMMANDS ...]]

positional arguments:
  commands

options:
  --exit, -x              Exit after sending the commands
  --verbose, -v           Display verbose server communication
  --url URL               The websocket url to connect (ws://localhost:8080/stream/)
  --user USER             The user name to connect with (guble-cli)
  --log-info              Log on INFO level (false)
  --log-debug             Log on DEBUG level (false)
```

## Commands in the client
In the running client, you can use the commands from the websocket api, e.g:
```
?           # prints some usage info
+ /foo/bar  # subscribe to the topic /foo/bar
+ /foo 0    # read from message 0 and subscribe to the topic /foo
+ /foo 0 5  # read messages 0-5 from /foo
+ /foo -5   # read the last 5 messages and subscribe to the topic /foo
- /foo      # cancel the subscription for /foo

> /foo         # send a message to /foo
> /foo/bar 42  # send a message to /foo/bar with publisherid 42
```



