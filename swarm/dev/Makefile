.PHONY: build cluster test

default: build

build:
	go build -o bin/swarm    github.com/plotozhu/MDCMainnet/cmd/swarm
	go build -o bin/geth     github.com/plotozhu/MDCMainnet/cmd/geth
	go build -o bin/bootnode github.com/plotozhu/MDCMainnet/cmd/bootnode

cluster: build
	scripts/boot-cluster.sh

test:
	go test -v github.com/plotozhu/MDCMainnet/swarm/...
