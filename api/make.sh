#!/bin/bash

# XXX: I wish that I were a Makefile

#apidirs=$(find ../../api -type d | cut -d/ -f4- | xargs)
protodirs=$(find . -name "*.proto" | xargs dirname | uniq)

if [[ $1 = "clean" ]]; then
	find . -name "*.pb.go" -exec rm {} \;
	exit 0
fi

for dir in $protodirs; do
	protoc --go_out=plugins=grpc:$GOPATH/src -I. $dir/*.proto
done
