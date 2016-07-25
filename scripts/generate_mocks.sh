#!/bin/bash -xe

if [ -z "$GOPATH" ]; then
      echo "Missing $GOPATH!";
      exit 1
fi


# replace in file if last operation was successful
function replace {
      FILE=$1; shift;
      while [ -n "$1" ]; do
            echo "Replacing: $1"
            sed -i "s/$1//g" $FILE
            shift
      done
}

MOCKGEN=$GOPATH/bin/mockgen

# server mocks
$MOCKGEN  -self_package server -package server \
      -destination server/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      Router
replace "server/mocks_server_gen_test.go" "server \"github.com\/smancke\/guble\/server\"" "server\."

$MOCKGEN -self_package server -package server \
      -destination server/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      MessageStore

$MOCKGEN -self_package server -package server \
      -destination server/mocks_server_kvstore_gen_test.go \
      github.com/smancke/guble/server/kvstore \
      KVStore

$MOCKGEN -self_package server -package server \
      -destination server/mocks_checker_gen_test.go \
      github.com/docker/distribution/health \
      Checker

$MOCKGEN -self_package server -package server \
      -destination server/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager

# client mocks
$MOCKGEN  -self_package client -package client \
      -destination client/mocks_client_gen_test.go \
      github.com/smancke/guble/client \
      WSConnection,Client
replace "client/mocks_client_gen_test.go" "client \"github.com\/smancke\/guble\/client\"" "client\."


# gcm mocks
$MOCKGEN -package gcm \
      -destination gcm/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      Router

$MOCKGEN -self_package gcm -package gcm \
      -destination gcm/mocks_server_kvstore_gen_test.go \
      github.com/smancke/guble/server/kvstore \
      KVStore

$MOCKGEN -self_package gcm -package gcm \
      -destination gcm/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      MessageStore

# gubled mocks
$MOCKGEN -package gubled \
      -destination gubled/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      Router

$MOCKGEN -self_package gubled -package gubled \
      -destination gubled/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager

$MOCKGEN -self_package gubled -package gubled \
      -destination gubled/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      MessageStore


# auth mocks
$MOCKGEN -self_package auth -package auth \
      -destination server/auth/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager
replace "server/auth/mocks_auth_gen_test.go" \
      "auth \"github.com\/smancke\/guble\/server\/auth\"" \
      "auth\."

# server/websocket mocks
$MOCKGEN  -self_package websocket -package websocket \
      -destination server/websocket/mocks_websocket_gen_test.go \
      github.com/smancke/guble/server/websocket \
      WSConnection
replace "server/websocket/mocks_websocket_gen_test.go" \
      "websocket \"github.com\/smancke\/server\/websocket\"" \
      "websocket\."

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      Router

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      MessageStore

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager

# Server/Rest Mocks
$MOCKGEN -package rest \
      -destination server/rest/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      Router
