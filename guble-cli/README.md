# The guble commandline client

This is the command line client for the guble messaging server. It is intended
demonstration and debugging use.

[![Build Status](https://api.travis-ci.org/smancke/guble.svg)](https://travis-ci.org/smancke/guble)


## Starting the client with docker 
The guble docker image has the command line client included. You can execute it within a running golang container and
connect to the server.
```
docker run -d --name guble smancke/guble
docker exec -it guble /go/bin/guble-cli
```


## Building with fo from source
```
	go get github.com/smancke/guble/guble-cli
	bin/guble-cli
```

## Start options
```
usage: guble-cli [--verbose] [--url URL] [--user USER] [--log-info] [--log-debug] [TOPICS [TOPICS ...]]

positional arguments:
  topics

options:
  --verbose, -v           Display verbose server communication
  --url URL               The websocket url to connect (ws://localhost:8080/stream/)
  --user USER             The user name to connect with (guble-cli)
  --log-info              Log on INFO level (false)
  --log-debug             Log on DEBUG level (false)
```

## Commands in the client
In the running client, you can use the commands from the websocket api, e.g:
```
?        # prints some usage info
+ /foo   # subscribe to topic /foo
- /foo   # unsubscribe from the topic /foo

> /foo   # send a message to /foo
{}       # with header {}
Hello    # and body Hello
```



