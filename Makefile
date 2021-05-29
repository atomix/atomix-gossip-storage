export CGO_ENABLED=0
export GO111MODULE=on

.PHONY: build

ifdef VERSION
STORAGE_VERSION := $(VERSION)
else
STORAGE_VERSION := latest
endif

all: build

build: # @HELP build the source code
build: deps
	GOOS=linux GOARCH=amd64 go build -gcflags=-trimpath=${GOPATH} -asmflags=-trimpath=${GOPATH} -o build/_output/atomix-gossip-storage-node ./cmd/atomix-gossip-storage-node
	GOOS=linux GOARCH=amd64 go build -gcflags=-trimpath=${GOPATH} -asmflags=-trimpath=${GOPATH} -o build/_output/atomix-gossip-storage-driver ./cmd/atomix-gossip-storage-driver
	GOOS=linux GOARCH=amd64 go build -gcflags=-trimpath=${GOPATH} -asmflags=-trimpath=${GOPATH} -o build/_output/atomix-gossip-storage-controller ./cmd/atomix-gossip-storage-controller

deps: # @HELP ensure that the required dependencies are in place
	go build -v ./...

test: # @HELP run the unit tests and source code validation
test: build license_check linters
	go test github.com/atomix/atomix-gossip-storage/...

coverage: # @HELP generate unit test coverage data
coverage: build linters license_check

linters: # @HELP examines Go source code and reports coding problems
	GOGC=50 golangci-lint run

license_check: # @HELP examine and ensure license headers exist
	./build/bin/license-check

proto: # @HELP build Protobuf/gRPC generated types
proto:
	docker run -it -v `pwd`:/go/src/github.com/atomix/atomix-gossip-storage \
		-w /go/src/github.com/atomix/atomix-gossip-storage \
		--entrypoint build/bin/compile_protos.sh \
		onosproject/protoc-go:stable

images: # @HELP build atomix storage controller Docker images
images: build
	docker build . -f build/atomix-gossip-storage-node/Dockerfile       -t atomix/atomix-gossip-storage-node:${STORAGE_VERSION}
	docker build . -f build/atomix-gossip-storage-driver/Dockerfile     -t atomix/atomix-gossip-storage-driver:${STORAGE_VERSION}
	docker build . -f build/atomix-gossip-storage-controller/Dockerfile -t atomix/atomix-gossip-storage-controller:${STORAGE_VERSION}

kind: images
	@if [ "`kind get clusters`" = '' ]; then echo "no kind cluster found" && exit 1; fi
	kind load docker-image atomix/atomix-gossip-storage-node:${STORAGE_VERSION}
	kind load docker-image atomix/atomix-gossip-storage-driver:${STORAGE_VERSION}
	kind load docker-image atomix/atomix-gossip-storage-controller:${STORAGE_VERSION}

push: # @HELP push atomix-gossip-node Docker image
	docker push atomix/atomix-gossip-storage-node:${STORAGE_VERSION}
	docker push atomix/atomix-gossip-storage-driver:${STORAGE_VERSION}
	docker push atomix/atomix-gossip-storage-controller:${STORAGE_VERSION}
