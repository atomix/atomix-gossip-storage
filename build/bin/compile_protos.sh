#!/bin/sh

proto_imports="./pkg:./test:${GOPATH}/src/github.com/gogo/protobuf:${GOPATH}/src/github.com/gogo/protobuf/protobuf:${GOPATH}/src"

protoc -I=$proto_imports --gogofaster_out=Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,import_path=atomix/protocol/gossip,plugins=grpc:pkg pkg/storage/*.proto
