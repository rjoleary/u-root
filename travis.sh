#!/bin/bash
set -e
export GOPATH=/home/travis/gopath
export CGO_ENABLED=0
GOFLAGS="-a -ldflags='-s'"

which go
ls -l cmds/*

(cd bb && go build $GOFLAGS && du -h bb && ./bb)
(cd scripts && go run ramfs.go -d -tmpdir=/tmp/u-root -removedir=false)
(cd cmds && go build $GOFLAGS ./...)
(cd cmds && go test $GOFLAGS ./...)
(cd cmds && go test $GOFLAGS -cover ./...)
(cd cmds && go test $GOFLAGS -race ./...)

go tool vet cmds uroot netlink memmap
go tool vet scripts/ramfs.go
sudo date
echo "Did it blend"

# TODO: why?
GOBIN=/tmp/u-root/ubin GOROOT=/tmp/u-root/go GOPATH=/tmp/u-root /tmp/u-root/go/bin/go build -x github.com/u-root/u-root/cmds/ip
