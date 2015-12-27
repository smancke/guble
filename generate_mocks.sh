#!/bin/bash -e

export GOPATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )/../../../.." && pwd )

set -x

$GOPATH/bin/mockgen  -self_package server -package server \
            github.com/smancke/guble/server \
            PubSubSource,MessageSink,WSConn,Startable,Stopable,SetRouter,SetMessageEntry,Endpoint,SetKVStore | sed -e 's/server "github.com\/smancke\/guble\/server"//' | sed -e 's/server\.//g' > $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go_ \
            && mv $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go_ $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go

$GOPATH/bin/mockgen  -self_package client -package client \
            github.com/smancke/guble/client \
            WSConnection | sed -e 's/client "github.com\/smancke\/guble\/client"//' | sed -e 's/server\.//g' > $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go_ \
            && mv $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go_ $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go

$GOPATH/bin/mockgen -package gcm \
            -destination $GOPATH/src/github.com/smancke/guble/gcm/mocks_server_gen_test.go \
            github.com/smancke/guble/server \
            PubSubSource

