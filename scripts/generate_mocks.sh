#!/bin/bash -xe

if [ -z "$GOPATH" ]; then
      echo "Missing $GOPATH!";
      exit 1
fi

# move file from source($1) to destination($2) if previous command exit was 0
function move_file {
      echo "Last exit error: $3"
      if [ $? -eq 0 ]; then
            mv $1 $2
      else
            rm $1
            exit 1
      fi
}

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
      PubSubSource,MessageSink,WSConnection,Startable,Stopable,SetRouter,SetMessageEntry,Endpoint,SetMessageStore
replace "server/mocks_server_gen_test.go" "server \"github.com\/smancke\/guble\/server\"" "server\."

$MOCKGEN -self_package server -package server \
      -destination server/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      MessageStore

# Client mocks
$MOCKGEN  -self_package client -package client \
      -destination client/mocks_client_gen_test.go \
      github.com/smancke/guble/client \
      WSConnection,Client
replace "client/mocks_client_gen_test.go" "client \"github.com\/smancke\/guble\/client\"" "client\."


# # GCM Mocks
$MOCKGEN -package gcm \
      -destination gcm/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      PubSubSource

$MOCKGEN -self_package gcm -package gcm \
      -destination gcm/mocks_store_gen_test.go \
      github.com/smancke/guble/store \
      KVStore

# # Gubled mocks
$MOCKGEN -package gubled \
      -destination gubled/mocks_server_gen_test.go \
      github.com/smancke/guble/server \
      PubSubSource