GO_PKGS=$(shell go list ./... | grep -v '/vendor/')

all: server

bin:
	@mkdir -p bin

run:
	go run main.go ${ARGS}

server: bin main.go
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/server

release: server

test:
	./test.sh

lint:
	@golint ./... | grep -v "should have comment" | grep -v "vendor/" | cat

vet:
	go tool vet `ls -l | grep -e '^d.*' | grep -v 'vendor' | awk '{print $$(NF)}' | xargs`

ci: test lint vet

clean:
	@rm -rf bin/*
	@echo clean done.

.PHONY: server release ci lint vet clean
