.PHONY: lint lint-all build-dummy install-deps protoc

FIRST_COMMIT ?= $(shell git rev-list --max-parents=0 HEAD)

lint:
	./build/bin/golangci-lint run --config ./.golangci.yml

builds-dummy:
	cd build && go build -o ./ibft1
	cd build && go build -o ./ibft2

lint-all:
	./build/bin/golangci-lint run --config ./.golangci.yml --new-from-rev=$(FIRST_COMMIT)

install-deps:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./build/bin v1.50.1

protoc:
	protoc --go_out=. --go-grpc_out=. ./messages/proto/messages.proto
