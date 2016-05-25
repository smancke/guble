#!/bin/bash -ex
# Require installation of: `github.com/wadey/gocovmerge`

cd $GOPATH/src/github.com/smancke/guble

rm -rf ./cov
mkdir cov

go test -v -covermode=atomic -coverprofile=./cov/client.out ./client
go test -v -covermode=atomic -coverprofile=./cov/gcm.out ./gcm
go test -v -covermode=atomic -coverprofile=./cov/guble-cli.out ./guble-cli
go test -v -covermode=atomic -coverprofile=./cov/gubled.out ./gubled
go test -v -covermode=atomic -coverprofile=./cov/protocol.out ./protocol
go test -v -covermode=atomic -coverprofile=./cov/server.out ./server
go test -v -covermode=atomic -coverprofile=./cov/store.out ./store

gocovmerge ./cov/*.out > full_cov.out
rm -rf ./cov
