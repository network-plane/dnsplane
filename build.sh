#!/bin/bash
BINDIR=$(dirname "$0")/bin
BUILDOS=$1
BUILDARCH=$2
CGO=0
CDIR=${PWD##*/} # to assign to a variable

echo Building "$CDIR"

if [ -z "$2" ]; then
    BUILDARCH="amd64"
fi

if [ -z "$1" ]; then
    BUILDOS="linux"
    echo "Statically Building in $BINDIR for $BUILDOS ($BUILDARCH)"
    GOOS=$BUILDOS GOARCH=$BUILDARCH CGO_ENABLED=$CGO go build -o "$BINDIR"/"$CDIR"

    BUILDOS="windows"
    echo "Statically Building in $BINDIR for $BUILDOS ($BUILDARCH)"
    GOOS=$BUILDOS GOARCH=$BUILDARCH CGO_ENABLED=$CGO go build -o "$BINDIR"/"$CDIR".exe

    BUILDOS="darwin"
    echo "Statically Building in $BINDIR for $BUILDOS ($BUILDARCH)"
    GOOS=$BUILDOS GOARCH=$BUILDARCH CGO_ENABLED=$CGO go build -o "$BINDIR"/"$CDIR"_darwin

    BUILDOS="darwin"
    BUILDARCH=arm64
    echo "Statically Building in $BINDIR for $BUILDOS ($BUILDARCH)"
    GOOS=$BUILDOS GOARCH=$BUILDARCH CGO_ENABLED=$CGO CGO_ENABLED=0 go build -o "$BINDIR"/"$CDIR"_darwin_arm64
else
    echo "Statically Building in $BINDIR for $BUILDOS ($BUILDARCH)"
    GOOS=$BUILDOS GOARCH=$BUILDARCH CGO_ENABLED=$CGO CGO_ENABLED=0 go build -o "$BINDIR"/"$CDIR"
fi
