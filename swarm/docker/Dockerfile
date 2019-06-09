FROM golang:1.11-alpine as builder

ARG VERSION

RUN apk add --update git gcc g++ linux-headers

RUN mkdir -p $GOPATH/src/github.com/plotozhu/MDCMainnet/wiredtiger && \
    cd $GOPATH/src/github.com/plotozhu && \
    git clone https://github.com/wiredtiger/wiredtiger.git && \
    cd $GOPATH/src/github.com/plotozhu/wiredtiger && \
    sh autogen.sh           &&  \
    ./configure --prefix=$GOPATH/src/github.com/plotozhu/MDCMainnet/wiredtiger
     make  && make install

RUN  cd $GOPATH/src/github.com/plotozhu && \
    git clone https://3823a1162e4a52f6e284610be5151c4204a900e7:@github.com/plotozhu/MDCMainnet && \
    cd $GOPATH/src/github.com/plotozhu/MDCMainnet && \
    git checkout ${VERSION} && \
    go install -ldflags "-X main.gitCommit=${VERSION}" ./cmd/swarm && \
    go install -ldflags "-X main.gitCommit=${VERSION}" ./cmd/swarm/swarm-smoke && \
    go install -ldflags "-X main.gitCommit=${VERSION}" ./cmd/swarm/global-store && \
    go install -ldflags "-X main.gitCommit=${VERSION}" ./cmd/geth


FROM alpine:3.8 as esc
RUN apk add iproute2
WORKDIR /
COPY --from=builder /go/bin/swarm /
COPY --from=builder /go/bin/geth /
COPY --from=builder /go/bin/swarm-smoke /
COPY --from=builder /go/bin/global-store /
ADD run.sh /run.sh
ADD run-smoke.sh /run-smoke.sh
ENTRYPOINT ["/config/run.sh"]
