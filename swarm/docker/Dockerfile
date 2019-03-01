FROM golang:1.11-alpine as builder

ARG VERSION

RUN apk add iproute2

FROM alpine:3.8
WORKDIR /
Add geth   /geth
Add swarm-smoke /swarm-smoke
Add swarm /swarm
ADD run.sh /run.sh
ADD run-smoke.sh /run-smoke.sh

ENTRYPOINT ["/config/run.sh"]
