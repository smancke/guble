#!/bin/bash -e

export GOPATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )/../../../.." && pwd )

set -x

$GOPATH/bin/mockgen  -self_package server -package server \
            github.com/smancke/guble/server \
            PubSubSource,MessageSink,WSConnection,Startable,Stopable,SetRouter,SetMessageEntry,Endpoint,SetKVStore,SetMessageStore | sed -e 's/server "github.com\/smancke\/guble\/server"//' | sed -e 's/server\.//g' | sed -e 's/client\.//g' > $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go_ \
            && mv $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go_ $GOPATH/src/github.com/smancke/guble/server/mocks_server_gen_test.go

$GOPATH/bin/mockgen -self_package server -package server \
            -destination $GOPATH/src/github.com/smancke/guble/server/mocks_store_gen_test.go \
            github.com/smancke/guble/store \
            MessageStore

$GOPATH/bin/mockgen  -self_package client -package client \
            github.com/smancke/guble/client \
            WSConnection,Client | sed -e 's/client "github.com\/smancke\/guble\/client"//' | sed -e 's/server\.//g'| sed -e 's/client\.//g' > $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go_ \
            && mv $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go_ $GOPATH/src/github.com/smancke/guble/client/mocks_client_gen_test.go

$GOPATH/bin/mockgen -package gcm \
            -destination $GOPATH/src/github.com/smancke/guble/gcm/mocks_server_gen_test.go \
            github.com/smancke/guble/server \
            PubSubSource

$GOPATH/bin/mockgen  -self_package auth -package auth github.com/smancke/guble/server/auth AccessManager | sed -e 's/auth "github.com\/smancke\/guble\/server\/auth"//' | sed -e 's/auth\.//g'>$GOPATH/src/github.com/smancke/guble/server/auth/mocks_auth_gen_test.go_ && mv $GOPATH/src/github.com/smancke/guble/server/auth/mocks_auth_gen_test.go_ $GOPATH/src/github.com/smancke/guble/server/auth/mocks_auth_gen_test.go


