#!/bin/bash -e

# export GOPATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )/../../../.." && pwd )

set -xe

# Server mocks
$GOPATH/bin/mockgen  -self_package server -package server \
            github.com/smancke/guble/server \
            PubSubSource,MessageSink,WSConnection,Startable,Stopable,SetRouter,SetMessageEntry,Endpoint,SetMessageStore | sed -e 's/server "github.com\/smancke\/guble\/server"//' | sed -e 's/server\.//g' > $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go_ \
            && mv $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go_ $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go

$GOPATH/bin/mockgen -self_package server -package server \
            -destination $GOPATH/src/github.com/smancke/guble/server/mocks_store_gen_test.go \
            github.com/smancke/guble/store \
            MessageStore

# Client mocks
$GOPATH/bin/mockgen  -self_package client -package client \
            github.com/smancke/guble/client \
            WSConnection,Client | sed -e 's/client "github.com\/smancke\/guble\/client"//' | sed -e 's/server\.//g' > $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go_ \
            && mv $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go_ $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go


# GCM Mocks
$GOPATH/bin/mockgen -package gcm \
            -destination $GOPATH/src/github.com/smancke/guble/gcm/mocks_server_gen_test.go \
            github.com/smancke/guble/server \
            PubSubSource

$GOPATH/bin/mockgen -self_package gcm -package gcm \
            -destination $GOPATH/src/github.com/smancke/guble/gcm/mocks_store_gen_test.go \
            github.com/smancke/guble/store \
            KVStore

