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
      github.com/cosminrentea/gobbler/server/router \
      Router &

$MOCKGEN -self_package service -package service \
      -destination server/service/mocks_checker_gen_test.go \
      github.com/docker/distribution/health \
      Checker &

# server/router mocks
$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router
replace "server/router/mocks_router_gen_test.go" "router \"github.com\/cosminrentea\/gobbler\/server\/router\"" "router\."

$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_store_gen_test.go \
      github.com/cosminrentea/gobbler/server/store \
      MessageStore &

$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_kvstore_gen_test.go \
      github.com/cosminrentea/gobbler/server/kvstore \
      KVStore &

$MOCKGEN -self_package router -package router \
      -destination server/router/mocks_checker_gen_test.go \
      github.com/docker/distribution/health \
      Checker &

# client mocks
$MOCKGEN  -self_package client -package client \
      -destination client/mocks_client_gen_test.go \
      github.com/cosminrentea/gobbler/client \
      WSConnection,Client
replace "client/mocks_client_gen_test.go" "client \"github.com\/cosminrentea\/gobbler\/client\"" "client\."

# server/apns mocks
$MOCKGEN -package apns \
      -destination server/apns/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router &

$MOCKGEN -package apns \
      -destination server/apns/mocks_kvstore_gen_test.go \
      github.com/cosminrentea/gobbler/server/kvstore \
      KVStore &

$MOCKGEN -package apns \
      -destination server/apns/mocks_connector_gen_test.go \
      github.com/cosminrentea/gobbler/server/connector \
      Sender,Request,Subscriber &

$MOCKGEN -package apns \
      -destination server/apns/mocks_pusher_gen_test.go \
      github.com/cosminrentea/gobbler/server/apns \
      Pusher &

# server/fcm mocks
$MOCKGEN -package fcm \
      -destination server/fcm/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router &

$MOCKGEN -self_package fcm -package fcm \
      -destination server/fcm/mocks_kvstore_gen_test.go \
      github.com/cosminrentea/gobbler/server/kvstore \
      KVStore &

$MOCKGEN -self_package fcm -package fcm \
      -destination server/fcm/mocks_store_gen_test.go \
      github.com/cosminrentea/gobbler/server/store \
      MessageStore &

$MOCKGEN -self_package fcm -package fcm \
      -destination server/fcm/mocks_gcm_gen_test.go \
      github.com/Bogh/gcm \
      Sender &

# server mocks
$MOCKGEN -package server \
      -destination server/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router &

$MOCKGEN -self_package server -package server \
      -destination server/mocks_store_gen_test.go \
      github.com/cosminrentea/gobbler/server/store \
      MessageStore &

$MOCKGEN -package server \
      -destination server/mocks_apns_pusher_gen_test.go \
      github.com/cosminrentea/gobbler/server/apns \
      Pusher &


# server/connector mocks
$MOCKGEN -self_package connector -package connector \
      -destination server/connector/mocks_connector_gen_test.go \
      github.com/cosminrentea/gobbler/server/connector \
      Connector,Sender,ResponseHandler,Manager,Queue,Request,Subscriber
replace "server/connector/mocks_connector_gen_test.go" \
      "connector \"github.com\/cosminrentea\/gobbler\/server\/connector\"" \
      "connector\."

$MOCKGEN -self_package connector -package connector \
      -destination server/connector/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router &

$MOCKGEN -self_package connector -package connector \
      -destination server/connector/mocks_kvstore_gen_test.go \
      github.com/cosminrentea/gobbler/server/kvstore \
      KVStore &

# server/websocket mocks
$MOCKGEN  -self_package websocket -package websocket \
      -destination server/websocket/mocks_websocket_gen_test.go \
      github.com/cosminrentea/gobbler/server/websocket \
      WSConnection
replace "server/websocket/mocks_websocket_gen_test.go" \
      "websocket \"github.com\/cosminrentea\/gobbler\/websocket\"" \
      "websocket\."

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router &

$MOCKGEN -self_package websocket -package websocket \
      -destination server/websocket/mocks_store_gen_test.go \
      github.com/cosminrentea/gobbler/server/store \
      MessageStore &

# server/rest Mocks
$MOCKGEN -package rest \
      -destination server/rest/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router &

# server/sms Mocks
$MOCKGEN -package sms \
      -destination server/sms/mocks_sender_gen_test.go \
      github.com/cosminrentea/gobbler/server/sms \
      Sender &

$MOCKGEN -package sms \
      -destination server/sms/mocks_router_gen_test.go \
      github.com/cosminrentea/gobbler/server/router \
      Router &

$MOCKGEN -package sms \
      -destination server/sms/mocks_kafka_producer_gen_test.go \
      github.com/cosminrentea/gobbler/server/kafka \
      Producer &

$MOCKGEN -self_package router -package sms \
      -destination server/sms/mocks_store_gen_test.go \
      github.com/cosminrentea/gobbler/server/store \
      MessageStore &

# server/configstring mocks
$MOCKGEN -package configstring \
      -destination server/configstring/mocks_kingpin_gen_test.go \
      gopkg.in/alecthomas/kingpin.v2 \
      Settings &

wait
