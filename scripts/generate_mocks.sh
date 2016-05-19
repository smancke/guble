#!/bin/bash -ex

if [ -z "$GOPATH" ]; then
      echo "Missing $GOPATH!";
      exit 1
fi

MOCKGEN=$GOPATH/bin/mockgen

# Server mocks
$MOCKGEN  -self_package server -package server \
      github.com/smancke/guble/server \
      PubSubSource,MessageSink,WSConnection,Startable,Stopable,SetRouter,SetMessageEntry,Endpoint,SetMessageStore \
      | sed -e 's/server "github.com\/smancke\/guble\/server"//' \
      | sed -e 's/server\.//g' \
      > server/mocks_server_gen_test.go_
mv server/mocks_server_gen_test.go_ server/mocks_server_gen_test.go

$MOCKGEN -self_package server -package server \
      -destination server/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      MessageStore

# Client mocks
$MOCKGEN  -self_package client -package client \
      github.com/smancke/guble/client \
      WSConnection,Client \
      | sed -e 's/client "github.com\/smancke\/guble\/client"//' \
      | sed -e 's/client\.//g' \
      > client/mocks_client_gen_test.go_
mv client/mocks_client_gen_test.go_ client/mocks_client_gen_test.go


# GCM Mocks
$MOCKGEN -package gcm \
      -destination gcm/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      PubSubSource

$MOCKGEN -self_package gcm -package gcm \
      -destination gcm/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      KVStore

# Gubled mocks
$MOCKGEN -package gubled \
      -destination gubled/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      PubSubSource