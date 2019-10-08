#!/bin/bash

kname=`uname -s`
kver=`uname -r`
karch=`uname -m`
gitcommit=`git rev-list --tags --max-count=1`
appver=`git describe --tags $gitcommit`
appbuild=`git rev-list ${appver}.. --count`

importroot="github.com/viert/xc/cli"

go build -o xc \
   -ldflags="-X $importroot.appVersion=$appver -X $importroot.appBuild=$appbuild -X $importroot.kernelName=$kname -X $importroot.kernelVersion=$kver -X $importroot.kernelArch=$karch" \
   cmd/xc/main.go
