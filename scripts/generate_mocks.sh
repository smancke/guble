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

# Server mocks
$MOCKGEN  -self_package server -package server \
      -destination server/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      Router,WSConnection,Module
replace "server/mocks_server_gen_test.go" "server \"github.com\/smancke\/guble\/server\"" "server\."

$MOCKGEN -self_package server -package server \
      -destination server/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      MessageStore

$MOCKGEN -self_package server -package server \
      -destination server/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager

# Client mocks
$MOCKGEN  -self_package client -package client \
      -destination client/mocks_client_gen_test.go \
      github.com/smancke/guble/client \
      WSConnection,Client
replace "client/mocks_client_gen_test.go" "client \"github.com\/smancke\/guble\/client\"" "client\."


# GCM Mocks
$MOCKGEN -package gcm \
      -destination gcm/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      Router

$MOCKGEN -self_package gcm -package gcm \
      -destination gcm/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      KVStore

# Gubled mocks
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


# Auth mocks
$MOCKGEN -self_package auth -package auth \
      -destination server/auth/mocks_auth_gen_test.go \
      github.com/smancke/guble/server/auth \
      AccessManager
replace "server/auth/mocks_auth_gen_test.go" \
      "auth \"github.com\/smancke\/guble\/server\/auth\"" \
      "auth\."