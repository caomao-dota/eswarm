// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package stream

import (
	"context"
	"errors"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/plotozhu/MDCMainnet/swarm/tracing"

	"github.com/plotozhu/MDCMainnet/metrics"
	"github.com/plotozhu/MDCMainnet/p2p/enode"
	"github.com/plotozhu/MDCMainnet/swarm/log"
	"github.com/plotozhu/MDCMainnet/swarm/network"
	"github.com/plotozhu/MDCMainnet/swarm/spancontext"
	"github.com/plotozhu/MDCMainnet/swarm/state"
	"github.com/plotozhu/MDCMainnet/swarm/storage"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

const (
	swarmChunkServerStreamName = "RETRIEVE_REQUEST"
	deliveryCap                = 32
)

var (
	processReceivedChunksCount    = metrics.NewRegisteredCounter("network.stream.received_chunks.count", nil)
	handleRetrieveRequestMsgCount = metrics.NewRegisteredCounter("network.stream.handle_retrieve_request_msg.count", nil)
	retrieveChunkFail             = metrics.NewRegisteredCounter("network.stream.retrieve_chunks_fail.count", nil)

	requestFromPeersCount     = metrics.NewRegisteredCounter("network.stream.request_from_peers.count", nil)
	requestFromPeersEachCount = metrics.NewRegisteredCounter("network.stream.request_from_peers_each.count", nil)
)

type Delivery struct {
	chunkStore   storage.SyncChunkStore
	kad          *network.Kademlia
	getPeer      func(enode.ID) *Peer
	receiptStore *state.ReceiptStore
	centralNodes []string
	mu           sync.RWMutex
	bzz          *network.Bzz
}

func NewDelivery(kad *network.Kademlia, chunkStore storage.SyncChunkStore, receiptStore *state.ReceiptStore) *Delivery {
	return &Delivery{
		chunkStore:   chunkStore,
		kad:          kad,
		receiptStore: receiptStore,
	}
}

// SwarmChunkServer implements Server
type SwarmChunkServer struct {
	deliveryC  chan []byte
	batchC     chan []byte
	chunkStore storage.ChunkStore
	currentLen uint64
	quit       chan struct{}
}

// NewSwarmChunkServer is SwarmChunkServer constructor
func NewSwarmChunkServer(chunkStore storage.ChunkStore) *SwarmChunkServer {
	s := &SwarmChunkServer{
		deliveryC:  make(chan []byte, deliveryCap),
		batchC:     make(chan []byte),
		chunkStore: chunkStore,
		quit:       make(chan struct{}),
	}
	go s.processDeliveries()
	return s
}

// processDeliveries handles delivered chunk hashes
func (s *SwarmChunkServer) processDeliveries() {
	var hashes []byte
	var batchC chan []byte
	for {
		select {
		case <-s.quit:
			return
		case hash := <-s.deliveryC:
			hashes = append(hashes, hash...)
			batchC = s.batchC
		case batchC <- hashes:
			hashes = nil
			batchC = nil
		}
	}
}

// SessionIndex returns zero in all cases for SwarmChunkServer.
func (s *SwarmChunkServer) SessionIndex() (uint64, error) {
	return 0, nil
}

// SetNextBatch
func (s *SwarmChunkServer) SetNextBatch(_, _ uint64) (hashes []byte, from uint64, to uint64, proof *HandoverProof, err error) {
	select {
	case hashes = <-s.batchC:
	case <-s.quit:
		return
	}

	from = s.currentLen
	s.currentLen += uint64(len(hashes))
	to = s.currentLen
	return
}

// Close needs to be called on a stream server
func (s *SwarmChunkServer) Close() {
	close(s.quit)
}

// GetData retrives chunk data from db store
func (s *SwarmChunkServer) GetData(ctx context.Context, key []byte) ([]byte, error) {
	chunk, err := s.chunkStore.Get(ctx, storage.Address(key))
	if err != nil {
		return nil, err
	}
	return chunk.Data(), nil
}

// RetrieveRequestMsg is the protocol msg for chunk retrieve requests
type RetrieveRequestMsg struct {
	Addr      storage.Address
	SkipCheck bool
	HopCount  uint8
}

//收到了某个节点来的查询数据的请求
func (d *Delivery) AttachBzz(bzz *network.Bzz) {
	d.bzz = bzz
}

//收到了某个节点来的查询数据的请求
func (d *Delivery) handleRetrieveRequestMsg(ctx context.Context, sp *Peer, req *RetrieveRequestMsg) error {
	log.Info("received request", "peer", sp.ID(), "hash", req.Addr)
	handleRetrieveRequestMsgCount.Inc(1)

	//记录
	var osp opentracing.Span
	ctx, osp = spancontext.StartSpan(
		ctx,
		"stream.handle.retrieve")

	s, err := sp.getServer(NewStream(swarmChunkServerStreamName, "", true))
	if err != nil {
		return err
	}
	streamer := s.Server.(*SwarmChunkServer)

	var cancel func()
	// TODO: do something with this hardcoded timeout, maybe use TTL in the future
	ctx = context.WithValue(ctx, "peer", sp.ID().String())
	ctx = context.WithValue(ctx, "hopcount", req.HopCount)
	ctx, cancel = context.WithTimeout(ctx, network.RequestTimeout)

	go func() {
		select {
		case <-ctx.Done():
		case <-streamer.quit:
		}
		cancel()
	}()

	go func() {
		defer osp.Finish()
		chunk, err := d.chunkStore.Get(ctx, req.Addr)
		if err != nil {
			retrieveChunkFail.Inc(1)
			log.Debug("ChunkStore.Get can not retrieve chunk", "peer", sp.ID().String(), "addr", req.Addr, "hopcount", req.HopCount, "err", err)
			return
		}
		if(len(chunk.Data()) == 10) {
			fmt.Printf("Data Len is 10, address %v",req.Addr)
		}
		if req.SkipCheck {
			syncing := false
			err = sp.Deliver(ctx, chunk, s.priority, syncing)
			if err != nil {
				log.Warn("ERROR in handleRetrieveRequestMsg", "err", err)
			}
			return
		}
		select {
		case streamer.deliveryC <- chunk.Address()[:]:
		case <-streamer.quit:
		}

	}()

	return nil
}

//Chunk delivery always uses the same message type....
type ChunkDeliveryMsg struct {
	Addr  storage.Address
	SData []byte // the stored chunk Data (incl size)
	peer  *Peer  // set in handleChunkDeliveryMsg
}

//...but swap accounting needs to disambiguate if it is a delivery for syncing or for retrieval
//as it decides based on message type if it needs to account for this message or not

//defines a chunk delivery for retrieval (with accounting)
type ChunkDeliveryMsgRetrieval ChunkDeliveryMsg

//defines a chunk delivery for syncing (without accounting)
type ChunkDeliveryMsgSyncing ChunkDeliveryMsg

// chunk delivery msg is response to retrieverequest msg
func (d *Delivery) handleChunkDeliveryMsg(ctx context.Context, sp *Peer, req *ChunkDeliveryMsg, replyReceipt bool) error {

	processReceivedChunksCount.Inc(1)

	// retrieve the span for the originating retrieverequest
	spanId := fmt.Sprintf("stream.send.request.%v.%v", sp.ID(), req.Addr)
	span := tracing.ShiftSpanByKey(spanId)

	go func() {
		if span != nil {
			defer span.(opentracing.Span).Finish()
		}

		req.peer = sp
		validData := d.chunkStore.Validate(req.SData,req.Addr)
		var err error
		if !validData  {
			err = errors.New("Chunk Data Invalid")
			fmt.Printf("Invalid Chunk Received: %v",req.Addr)
		}else {
			err = d.chunkStore.Put(ctx, storage.NewChunk(req.Addr, req.SData))
		}

		if err != nil {
			if err == storage.ErrChunkInvalid {
				// we removed this log because it spams the logs
				// TODO: Enable this log line
				// log.Warn("invalid chunk delivered", "peer", sp.ID(), "chunk", req.Addr, )
				req.peer.Drop(err)
			}
		}
		if replyReceipt && d.bzz != nil { //TODO 优化每隔10帧或是10秒发送一次收据
			hs, _ := d.bzz.GetOrCreateHandshake(sp.ID())
			//用最低的优先级，发送一个收据
			receipt, err := d.receiptStore.OnNodeChunkReceived(hs.Account)
			if err == nil {
				err = sp.SendPriority(ctx, &ReceiptsMsg{receipt.Account, uint32(receipt.Stime.Unix()), receipt.Amount, receipt.Sign}, Mid)
				if err != nil {
					log.Warn("send receipt failed", "peer", sp.ID(), "error", err)
				}

			} else {
				log.Warn("create receipt failed", "peer", sp.ID(), "error", err)
			}
		}

	}()
	return nil
}

// RequestFromPeers sends a chunk retrieve request to
// 发送一个chunk读取请求，ctx保存超时之类的信息，req是一个请求指令，包含了源地址和数据哈希，
func (d *Delivery) RequestFromPeers(ctx context.Context, req *network.Request) (*enode.ID, chan struct{}, error) {
	requestFromPeersCount.Inc(1)
	var sp *Peer
	spID := req.Source

	if spID != nil { //有直接的请求地址可以取
		sp = d.getPeer(*spID)
		if sp == nil {
			return nil, nil, fmt.Errorf("source peer %v not found", spID.String())
		}
	} else {
		//从KAD网络中，由近及远找到一个节点，向这个节点请求查询数据
		//TODO 需要理解，为什么不是从这个地址所在的桶里去查找，而是从近到远的找
		//比较容易理解的时，由于近的桶里的节点是直接连接好的，不需要重新连接，因此从最近的桶里找快一些，这个后面优化的时候分析一下
		d.kad.EachConn(req.Addr[:], 255, func(p *network.Peer, po int) bool {
			id := p.ID()
			if p.LightNode {
				// skip light nodes
				return true
			}
			if req.SkipPeer(id.String()) {
				log.Trace("Delivery.RequestFromPeers: skip peer", "peer id", id)
				return true
			}
			sp = d.getPeer(id)
			// sp is nil, when we encounter a peer that is not registered for delivery, i.e. doesn't support the `stream` protocol
			if sp == nil {
				return true
			}
			spID = &id
			return false
		})
		if sp == nil {
			return nil, nil, errors.New("no peer found")
		}
	}

	// setting this value in the context creates a new span that can persist across the sendpriority queue and the network roundtrip
	// this span will finish only when delivery is handled (or times out)
	ctx = context.WithValue(ctx, tracing.StoreLabelId, "stream.send.request")
	ctx = context.WithValue(ctx, tracing.StoreLabelMeta, fmt.Sprintf("%v.%v", sp.ID(), req.Addr))
	err := sp.SendPriority(ctx, &RetrieveRequestMsg{
		Addr:      req.Addr,
		SkipCheck: req.SkipCheck,
		HopCount:  req.HopCount,
	}, Top)
	if err != nil {
		return nil, nil, err
	}
	requestFromPeersEachCount.Inc(1)

	return spID, sp.quit, nil
}
func (d *Delivery) UpdateNodes(nodes []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.centralNodes = nodes
}

func (d *Delivery) GetDataFromCentral(ctx context.Context, address storage.Address) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	nodes_count := len(d.centralNodes)
	if nodes_count > 0 {
		//一个节点，一个小时内，总是只对一个中心节点取数据，这样提高收据合并的效率
		nodeIndex := (time.Now().Hour() + int(d.receiptStore.Account()[0])) % nodes_count
		times := 0
		go func() {
			for times < nodes_count {
				//	node,err := enode.ParseV4(d.centralNodes[nodeIndex])
				//	if err == nil {
				client := http.Client{Timeout: 5 * time.Second}
				resp, err := client.Get(d.centralNodes[nodeIndex] + "/chunk:/" + address.Hex()) //这个是阻塞型的

				if err == nil && resp.StatusCode == 200 {

					buffer, err := ioutil.ReadAll(resp.Body)

					if err == nil {

						d.handleChunkDeliveryMsg(ctx, nil, &ChunkDeliveryMsg{address, buffer, nil}, false)
						break
					}

					//}
				}
				nodeIndex = (nodeIndex + 1) % nodes_count
			}

		}()
	}

}

func (d *Delivery) handleReceiptsMsg(sp *Peer, receipt *ReceiptsMsg) error {

	return d.receiptStore.OnNewReceipt(&state.Receipt{state.ReceiptBody{receipt.PA, time.Unix(int64(receipt.STime), 0), receipt.AMount}, receipt.Sig})

}
