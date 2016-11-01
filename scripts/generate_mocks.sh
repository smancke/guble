#!/bin/bash -xe

# Prerequisites: mockgen should be installed
#   go get github.com/golang/mock/mockgen

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

# server/service mocks
$MOCKGEN  -self_package service -package service \
      -destination server/service/mocks_router_gen_test.go \
      github.com/smancke/guble/server/router \
      Router

$MOCKGEN -self_package service -package service \
      -destination server/service/mocks_checker_gen_test.go \
      github.com/docker/distribution/health \
      Checker

# server/router mocks
$MOCKGEN  -self_package router -package router \
      -destination server/router/mocks_router_gen_test.go \
      github.com/smancke/guble/server/router \
      Router
replace "server/router/mocks_router_gen_test.go" "router \"github.com\/smancke\/guble\/server\/router\"" "router\."

$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_store_gen_test.go \
      github.com/smancke/guble/server/store \
      MessageStore

$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_kvstore_gen_test.go \
      github.com/smancke/guble/server/kvstore \
      KVStore

$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager

$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_checker_gen_test.go \
      github.com/docker/distribution/health \
      Checker

# client mocks
$MOCKGEN  -self_package client -package client \
      -destination client/mocks_client_gen_test.go \
      github.com/smancke/guble/client \
      WSConnection,Client
replace "client/mocks_client_gen_test.go" "client \"github.com\/smancke\/guble\/client\"" "client\."


# server/fcm mocks
$MOCKGEN -package fcm \
      -destination server/fcm/mocks_router_gen_test.go \
      github.com/smancke/guble/server/router \
      Router

$MOCKGEN -self_package fcm -package fcm \
      -destination server/fcm/mocks_kvstore_gen_test.go \
      github.com/smancke/guble/server/kvstore \
      KVStore

$MOCKGEN -self_package fcm -package fcm \
      -destination server/fcm/mocks_store_gen_test.go \
      github.com/smancke/guble/server/store \
      MessageStore

$MOCKGEN -self_package fcm -package fcm \
      -destination server/fcm/mocks_gcm_gen_test.go \
      github.com/Bogh/gcm \
      Sender

# server mocks
$MOCKGEN -package server \
      -destination server/mocks_router_gen_test.go \
      github.com/smancke/guble/server/router \
      Router

$MOCKGEN -self_package server -package server \
      -destination server/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager

$MOCKGEN -self_package server -package server \
      -destination server/mocks_store_gen_test.go \
      github.com/smancke/guble/server/store \
      MessageStore


# server/auth mocks
$MOCKGEN -self_package auth -package auth \
      -destination server/auth/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager
replace "server/auth/mocks_auth_gen_test.go" \
      "auth \"github.com\/smancke\/guble\/server\/auth\"" \
      "auth\."

# server/connector mocks
$MOCKGEN -self_package connector -package connector \
      -destination server/connector/mocks_connector_gen_test.go \
      github.com/smancke/guble/server/connector \
      Connector,Sender,ResponseHandler,Manager,Queue,Request,Subscriber
replace "server/connector/mocks_connector_gen_test.go" \
      "connector \"github.com\/smancke\/guble\/server\/connector\"" \
      "connector\."

$MOCKGEN -self_package connector -package connector \
      -destination server/connector/mocks_router_gen_test.go \
      github.com/smancke/guble/server/router \
      Router


# server/websocket mocks
$MOCKGEN  -self_package websocket -package websocket \
      -destination server/websocket/mocks_websocket_gen_test.go \
      github.com/smancke/guble/server/websocket \
      WSConnection
replace "server/websocket/mocks_websocket_gen_test.go" \
      "websocket \"github.com\/smancke\/server\/websocket\"" \
      "websocket\."

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_router_gen_test.go \
      github.com/smancke/guble/server/router \
      Router

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_store_gen_test.go \
      github.com/smancke/guble/server/store \
      MessageStore

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager

# server/rest Mocks
$MOCKGEN -package rest \
      -destination server/rest/mocks_router_gen_test.go \
      github.com/smancke/guble/server/router \
      Router
