build: build-linux build-macos build-linux-server build-macos-server

build-dirs:
	mkdir -p builds/linux
	mkdir -p builds/macos

build-linux: build-dirs
	docker run --rm -v `pwd`:/go/src/github.com/virtru/cork -w /go/src/github.com/virtru/cork golang:1.8 go build -o builds/linux/cork

build-linux-server: build-dirs
	docker run --rm -v `pwd`:/go/src/github.com/virtru/cork -w /go/src/github.com/virtru/cork/server golang:1.8 go build -o ../builds/linux/cork-server

build-macos: build-dirs
	docker run --rm -e GOOS=darwin -v `pwd`:/go/src/github.com/virtru/cork -w /go/src/github.com/virtru/cork golang:1.8 go build -o builds/macos/cork

build-macos-server: build-dirs
	docker run --rm -e GOOS=darwin -v `pwd`:/go/src/github.com/virtru/cork -w /go/src/github.com/virtru/cork/server golang:1.8 go build -o ../builds/macos/cork-server

test-cork-server: build-linux-server
	docker build -f Dockerfile.test -t test-cork-server .

build-client-test:
	cd client_test && go build -o client-test .

base-cork-server: build-linux-server
	docker build -f Dockerfile.base -t virtru/base-cork-server .
	docker tag virtru/base-cork-server virtru/base-cork-server:xenial

proto-go:
	protoc -I protocol/ protocol/cork.proto --go_out=plugins=grpc:protocol

proto-py:
	python -m grpc_tools.protoc -I./protocol --python_out=client_test --grpc_python_out=client_test protocol/cork.proto

test:
	base=$(echo $PWD | sed "s|$GOPATH/src/||") go test $(go list ./... | grep -v vendor | sed "s|$base/|./|")
