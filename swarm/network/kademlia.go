// Copyright 2017 The go-ethereum Authors
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

package network

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/plotozhu/MDCMainnet/p2p"
	"github.com/plotozhu/MDCMainnet/p2p/enode"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/plotozhu/MDCMainnet/common"
	"github.com/plotozhu/MDCMainnet/swarm/log"
	"github.com/plotozhu/MDCMainnet/swarm/pot"
	sv "github.com/plotozhu/MDCMainnet/swarm/version"
)

/*

Taking the proximity order relative to a fix point x classifies the points in
the space (n byte long byte sequences) into bins. Items in each are at
most half as distant from x as items in the previous bin. Given a sample of
uniformly distributed items (a hash function over arbitrary sequence) the
proximity scale maps onto series of subsets with cardinalities on a negative
exponential scale.

It also has the property that any two item belonging to the same bin are at
most half as distant from each other as they are from x.

If we think of random sample of items in the bins as connections in a network of
interconnected nodes then relative proximity can serve as the basis for local
decisions for graph traversal where the task is to find a route between two
points. Since in every hop, the finite distance halves, there is
a guaranteed constant maximum limit on the number of hops needed to reach one
node from the other.
*/

var Pof = pot.DefaultPof(256)
const MaxPoBin = 16
// KadParams holds the config params for Kademlia
type KadParams struct {
	// adjustable parameters
	MaxProxDisplay    int   // number of rows the table shows
	NeighbourhoodSize int   // nearest neighbour core minimum cardinality
	MinBinSize        int   // minimum number of peers in a row
	MaxBinSize        int   // maximum number of peers in a row before pruning
	RetryInterval     int64 // initial interval before a peer is first redialed
	RetryExponent     int   // exponent to multiply retry intervals with
	MaxRetries        int   // maximum number of redial attempts
	// function to sanction or prevent suggesting a peer
	Reachable func(*BzzAddr) bool `json:"-"`
}

// NewKadParams returns a params struct with default values
func NewKadParams() *KadParams {
	return &KadParams{
		MaxProxDisplay:    16,
		NeighbourhoodSize: 2,
		MinBinSize:        2,
		MaxBinSize:        5,
		RetryInterval:     4200000000, // 4.2 sec
		MaxRetries:        42,
		RetryExponent:     2,
	}
}
type UDEF []byte

// Kademlia is a table of live peers and a db of known peers (node records)
type Kademlia struct {
	lock       sync.RWMutex
	*KadParams          // Kademlia configuration parameters
	base       []byte   // immutable baseaddress of the table
	addrs      *pot.Pot // pots container for known peer addresses
	conns      *pot.Pot // pots container for live peer connections
	lconns     *pot.Pot // pots container for light
	depth      uint8    // stores the last current depth of saturation
	nDepth     int      // stores the last neighbourhood depth
	nDepthC    chan int // returned by DepthC function to signal neighbourhood depth change
	peerChangedC chan int // returned by AddrCountC function to signal peer count change
	peers      map[string]bool
	filter     *p2p.FilterChain
	blacklist  *lru.Cache

}

// NewKademlia creates a Kademlia table for base address addr
// with parameters as in params
// if params is nil, it uses default values
func NewKademlia(addr []byte, params *KadParams) *Kademlia {
	if params == nil {
		params = NewKadParams()
	}
	blacklist,_:= lru.New(1000)
	return &Kademlia{
		base:      addr,
		KadParams: params,
		addrs:     pot.NewPot(nil, 0),
		conns:     pot.NewPot(nil, 0),
		lconns:    pot.NewPot(nil, 0),
		peers:     make(map[string]bool),
		blacklist:blacklist,
		nDepth:   -1,

	}
}

// entry represents a Kademlia table entry (an extension of BzzAddr)
type entry struct {
	*BzzAddr
	conn    *Peer
	seenAt  time.Time
	retries int
	lastRetry time.Time
	connectedAt time.Time
}

// newEntry creates a kademlia peer from a *Peer
func newEntry(p *BzzAddr) *entry {
	return &entry{
		BzzAddr: p,
		seenAt:  time.Now(),
		connectedAt: time.Unix(0,0),
	}
}

// Label is a short tag for the entry for debug
func Label(e *entry) string {
	return fmt.Sprintf("%s (%d)", e.Hex()[:4], e.retries)
}

// Hex is the hexadecimal serialisation of the entry address
func (e *entry) Hex() string {
	return fmt.Sprintf("%x", e.Address())
}
func (k *Kademlia) SetFilter(chain *p2p.FilterChain )  {
	k.filter = chain
	k.filter.AddFilter(k)
}
func (k *Kademlia) IsBlocked(id enode.ID,inbound bool) bool{
	 idStr := fmt.Sprintf("%v-%v",id,inbound)
	val,ok := k.blacklist.Get(idStr)
	if ok && !inbound && !time.Now().After(val.(p2p.BlackItem).DiscTime) {
		return true
	} else if inbound{
		bucketFull := false
		k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
			if !reflect.ValueOf(val).IsNil() {
				e := val.(*entry)

				if e.ID() == id {
					log.Info(" test for bucket","id",id,"bucket",po,"size",size)
					bucketFull = size >= k.MaxBinSize
					return false
				} else { //桶里连接满了，找下一个po吧
					return true
				}
			}else{
				return true
			}

		})
	}
	return false
}
func (k *Kademlia) CanSendPing(id enode.ID) bool{
	return true;
}
func (k *Kademlia) CanProcPing(id enode.ID) bool{
	return true;
}
func (k *Kademlia) CanProcPong(id enode.ID) bool{
	return true;
}
func (k *Kademlia) CanStartConnect(id enode.ID) bool{
	return true;
}
func (k *Kademlia)ShouldAcceptConn(id enode.ID) bool{
	return true;
}

func (k *Kademlia)ClearDelay(id enode.ID,inbound bool){

	k.blacklist.Remove(fmt.Sprintf("%v-%v",id,inbound))
}
func (k *Kademlia)SetDelay(id enode.ID,inbound bool ) {
	idStr := fmt.Sprintf("%v-%v",id,inbound)
	val,exist :=k.blacklist.Get(idStr)
	item := p2p.BlackItem{time.Now().Add(30*time.Second),1}
	if exist {
		item = val.(p2p.BlackItem)
		item.DiscTime = time.Now().Add(time.Duration(30*item.DiscCount)*time.Second)
		if item.DiscCount < 60 {
			item.DiscCount++
		}

	}
	log.Info("block peer","peer",idStr,"time to",item.DiscTime)


	k.blacklist.Add(idStr,item)

}
// Register enters each address as kademlia peer record into the
// database of known peer addresses
func (k *Kademlia) Register(peers ...*BzzAddr ) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	var known, size int

	for _, p := range peers {
		//log.Info("kad register","id",p.ID(),"register/unregister",toRegister,"Uaddr:",p.String())
		//log.Trace("kademlia trying to register", "addr", p)
		// error if self received, peer should know better
		// and should be punished for this
		if bytes.Equal(p.Address(), k.base) {
			return fmt.Errorf("add peers: %x is self", k.base)
		}
		var found bool
		k.addrs, _, found, _ = pot.Swap(k.addrs, p, Pof, func(v pot.Val) pot.Val {
			// if not found
			if v == nil {
				log.Trace("registering new peer", "addr", p)
				// insert new offline peer into conns

					return newEntry(p)


			}

				e := v.(*entry)

				// if underlay address is different, still add
				if !bytes.Equal(e.BzzAddr.UAddr, p.UAddr) {
					log.Trace("underlay addr is different, so add again", "new", p, "old", e.BzzAddr)
					// insert new offline peer into conns
					return newEntry(p)
				}

				//	log.Trace("found among known peers, underlay addr is same, do nothing", "new", p, "old", e.BzzAddr)

				return v


		})
		if found {
			known++
		}
		size++
	}

	//k.sendNeighbourhoodDepthChange()
	return nil
}
func (k *Kademlia)GetIntendBinInfo(nodeAddr *BzzAddr) (int,int){

	bin,_ := Pof(k.conns,nodeAddr,0)
//	bin = int(math.Log2(float64(bin)))
	return k.conns.SizeOfBin(bin),bin


}
func (k *Kademlia) __SuggestPeer() (suggestedPeer *BzzAddr, saturationDepth int, changed bool) {
	k.lock.Lock()
	defer k.lock.Unlock()

	sizeOfBin := make([]int,MaxPoBin)
	//first find connected count for each bin
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		sizeOfBin[po] = size
		return true
	})

	//找到最深的size >= 2的桶
	saturationDepth = 0
	connsOverSaturation := 0
	for i := 0; i < MaxPoBin; i++{
		if sizeOfBin[i] < k.MinBinSize  {
			saturationDepth = i
			break;
		}
	}
	for i := saturationDepth; i< MaxPoBin; i++ {
		connsOverSaturation += sizeOfBin[i]
	}
	//如果saturationDepth之上的所有节点不满足要求，那么降一格（降低后肯定满足要求了）
	//假设桶如下 saturationDepth 应该是 3
	//   0   1   2   3   4   5   6   7 .....
	// | 2 | 3 | 4 | 0 | 1 | 1 | 0 | 0  ....

	//假设桶如下 saturationDepth 应该是 2
	//   0   1   2   3   4   5   6   7 .....
	// | 2 | 3 | 4 | 0 | 1 | 0 | 0 | 0  ....

	//假设桶如下 saturationDepth 应该是 1
	//   0   1   2   3   4   5   6   7 .....
	// | 2 | 0 | 4 | 0 | 1 | 0 | 0 | 0  ....

	if connsOverSaturation < k.MinBinSize {
		if saturationDepth > 0 {
			saturationDepth --;
		}
	}
	targetPO := 0
	//寻找suggest peer
	suggestedPeer = nil

	//从优先满足广度，直至广度达到 k.MinBinSize+1
	k.addrs.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		if sizeOfBin[po] < k.MinBinSize+1 {
			return f(func(val pot.Val) bool {
				e := val.(*entry)
				_,ok := k.peers[string(e.UAddr)]

				if k.callable(e) && !ok {

					targetPO = po
					suggestedPeer = e.BzzAddr
					return false
				}
				return true
			})
		}else{ //桶里连接满了，找下一个po吧
			return true
		}

	})


	if suggestedPeer != nil{
		log.Trace("Suggested peer:","po",targetPO,"oaddr",common.Bytes2Hex(suggestedPeer.Over()))
	}/*else{
		//没有节点用可了，把标识成不能查询的恢复起来
		k.addrs.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
			return f(func(val pot.Val) bool {
				e := val.(*entry)
				_,ok := k.peers[string(e.UAddr)]
				if !ok {
					e.lastRetry = time.Unix(0,0)
					return true
				}
				return true
			})
		})
	}*/

	if uint8(saturationDepth) < k.depth {
		k.depth = uint8(saturationDepth)
		return suggestedPeer, saturationDepth, true
	}
	//在on的时候会改变depth，所以现在是不可能会变得saturationDepth > k.depth的
	return suggestedPeer, 0, false
}
// SuggestPeer returns an unconnected peer address as a peer suggestion for connection
// 这么简单的目标，写出这么复杂的逻辑.......
func (k *Kademlia) SuggestPeer() (suggestedPeer *BzzAddr, saturationDepth int, changed bool) {
	//log.Info("lock Sp")
	k.lock.Lock()
	//defer log.Info("unlock Sp")
	defer  k.lock.Unlock() ;
	radius := neighbourhoodRadiusForPot(k.conns, k.NeighbourhoodSize, k.base)
	// collect undersaturated bins in ascending order of number of connected peers
	// and from shallow to deep (ascending order of PO)
	// insert them in a map of bin arrays, keyed with the number of connected peers
	saturation := make(map[int][]int)
	var lastPO int       // the last non-empty PO bin in the iteration
	saturationDepth = -1 // the deepest PO such that all shallower bins have >= k.MinBinSize peers
	var pastDepth bool   // whether po of iteration >= depth
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		// process skipped empty bins
		for ; lastPO < po; lastPO++ {
			// find the lowest unsaturated bin
			if saturationDepth == -1 {
				saturationDepth = lastPO
			}
			// if there is an empty bin, depth is surely passed
			pastDepth = true
			saturation[0] = append(saturation[0], lastPO)
		}
		lastPO = po + 1
		// past radius, depth is surely passed
		if po >= radius {
			pastDepth = true
		}
		// beyond depth the bin is treated as unsaturated even if size >= k.MinBinSize
		// in order to achieve full connectivity to all neighbours
		if pastDepth && size >= k.MinBinSize {
			size = k.MinBinSize - 1
		}
		// process non-empty unsaturated bins
		if size < k.MinBinSize {
			// find the lowest unsaturated bin
			if saturationDepth == -1 {
				saturationDepth = po
			}
			saturation[size] = append(saturation[size], po)
		}
		return true
	})
	// to trigger peer requests for peers closer than closest connection, include
	// all bins from nearest connection upto nearest address as unsaturated
	var nearestAddrAt int
	k.addrs.EachNeighbour(k.base, Pof, func(_ pot.Val, po int) bool {
		nearestAddrAt = po
		return false
	})
	// including bins as size 0 has the effect that requesting connection
	// is prioritised over non-empty shallower bins
	for ; lastPO <= nearestAddrAt; lastPO++ {
		saturation[0] = append(saturation[0], lastPO)
	}
	// all PO bins are saturated, ie., minsize >= k.MinBinSize, no peer suggested
	if len(saturation) == 0 {
		return nil, 0, false
	}
	targetPO := 0
	// find the first callable peer in the address book
	// starting from the bins with smallest size proceeding from shallow to deep
	// for each bin (up until neighbourhood radius) we find callable candidate peers
	for size := 0; size < k.MinBinSize && suggestedPeer == nil; size++ {
		bins, ok := saturation[size]
		if !ok {
			// no bin with this size
			continue
		}
		cur := 0
		curPO := bins[0]
		k.addrs.EachBin(k.base, Pof, curPO, func(po, _ int, f func(func(pot.Val) bool) bool) bool {
			curPO = bins[cur]
			// find the next bin that has size size
			if curPO == po {
				cur++
			} else {
				// skip bins that have no addresses
				for ; cur < len(bins) && curPO < po; cur++ {
					curPO = bins[cur]
				}
				if po < curPO {
					cur--
					return true
				}
				// stop if there are no addresses
				if curPO < po {
					return false
				}
			}
			// curPO found
			// find a callable peer out of the addresses in the unsaturated bin
			// stop if found
			f(func(val pot.Val) bool {
				e := val.(*entry)
				ok := k.filter == nil || !k.filter.IsBlocked(e.ID(),false)
				if k.callable(e) &&  ok{
					targetPO = po

					suggestedPeer = e.BzzAddr
					return false
				}
				return true
			})
			return cur < len(bins) && suggestedPeer == nil
		})
	}
	if suggestedPeer != nil{
		log.Info("Suggested peer:","uaddr",string(suggestedPeer.UAddr),"po",targetPO)
	}

	if uint8(saturationDepth) < k.depth {
		k.depth = uint8(saturationDepth)
		return suggestedPeer, saturationDepth, true
	}
	return suggestedPeer, 0, false
}

// On inserts the peer as a kademlia peer into the live peers
func (k *Kademlia) On(p *Peer) (depth uint8, changed bool,err error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ins bool
	 changed  = false
	 k.peers[string(p.UAddr)] = true
	if p.NodeType() != enode.NodeTypeLight {
		po := int(0)

		k.conns, po, _, _ = pot.Swap(k.conns, p, Pof, func(v pot.Val) pot.Val {
			// if not found live
			if v == nil {

				if k.conns.SizeOfBin(po) < k.KadParams.MaxBinSize {
					ins = true
					// insert new online peer into conns
					return p
				}else {
					err = errors.New(fmt.Sprintf("Full bucket id:=%v  po:=%v",common.Bytes2Hex(p.OAddr),po))
					return v
				}

			}
			// found among live peers, do nothing
			return v
		})


		if ins  {
			a := newEntry(p.BzzAddr)
			a.conn = p
			a.connectedAt = time.Now()
			// insert new online peer into addrs
			k.addrs, _, _, _ = pot.Swap(k.addrs, p, Pof, func(v pot.Val) pot.Val {
				return a
			})
			// send new address count value only if the peer is inserted
			log.Info("Update addr counts","size", k.addrs.Size(),"channel",k.peerChangedC != nil)
			if k.peerChangedC != nil {
				k.peerChangedC <- k.conns.Size()
			}
		}

		log.Trace(k.string())
		// calculate if depth of saturation changed
		depth := uint8(k.Saturation())

		if depth != k.depth {
			changed = true
			k.depth = depth
		}
		k.sendNeighbourhoodDepthChange()

	}else{
		k.lconns, _, _, _ = pot.Swap(k.lconns, p, Pof, func(v pot.Val) pot.Val {
			// if not found live
			if v == nil {
				// insert new online peer into conns
				return p
			}
			// found among live peers, do nothing
			return v
		})
	}

	return k.depth, changed,err

}

// NeighbourhoodDepthC returns the channel that sends a new kademlia
// neighbourhood depth on each change.
// Not receiving from the returned channel will block On function
// when the neighbourhood depth is changed.
// TODO: Why is this exported, and if it should be; why can't we have more subscribers than one?
func (k *Kademlia) NeighbourhoodDepthC() <-chan int {
	k.lock.Lock()
	defer k.lock.Unlock()
	if k.nDepthC == nil {
		k.nDepthC = make(chan int)
	}
	return k.nDepthC
}

// CloseNeighbourhoodDepthC closes the channel returned by
// NeighbourhoodDepthC and stops sending neighbourhood change.
func (k *Kademlia) CloseNeighbourhoodDepthC() {
	k.lock.Lock()
	defer k.lock.Unlock()

	if k.nDepthC != nil {
		close(k.nDepthC)
		k.nDepthC = nil
	}
}

// sendNeighbourhoodDepthChange sends new neighbourhood depth to k.nDepth channel
// if it is initialized.
func (k *Kademlia) sendNeighbourhoodDepthChange() {
	// nDepthC is initialized when NeighbourhoodDepthC is called and returned by it.
	// It provides signaling of neighbourhood depth change.
	// This part of the code is sending new neighbourhood depth to nDepthC if that condition is met.
	if k.nDepthC != nil {

		nDepth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
		if nDepth != k.nDepth {
			k.nDepth = nDepth
			log.Info("new neighbor","depth",nDepth)
			k.nDepthC <- nDepth
		}
	}
}

// AddrCountC returns the channel that sends a new
// address count value on each change.
// Not receiving from the returned channel will block Register function
// when address count value changes.
func (k *Kademlia) PeerChangedC() <-chan int {
	k.lock.Lock()
	defer k.lock.Unlock()

	if k.peerChangedC == nil {
		k.peerChangedC = make(chan int)
	}
	return k.peerChangedC
}

// CloseAddrCountC closes the channel returned by
// AddrCountC and stops sending address count change.
func (k *Kademlia) CloseAddrCountC() {
	k.lock.Lock()
	defer k.lock.Unlock()

	if k.peerChangedC != nil {
		close(k.peerChangedC)
		k.peerChangedC = nil
	}
}

// Off removes a peer from among live peers
func (k *Kademlia) Off(p *Peer) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var del bool
	delete (k.peers,string(p.UAddr))
	k.SetDelay(p.ID(),false)
	if p.NodeType() != enode.NodeTypeLight {
		k.addrs, _, _, _ = pot.Swap(k.addrs, p, Pof, func(v pot.Val) pot.Val {
			// v cannot be nil, must check otherwise we overwrite entry
			if v == nil {
				panic(fmt.Sprintf("connected peer not found %v", p))
			}
			del = true
			return newEntry(p.BzzAddr)
		})

		if del {
			k.conns, _, _, _ = pot.Swap(k.conns, p, Pof, func(_ pot.Val) pot.Val {
				// v cannot be nil, but no need to check
				return nil
			})
			// send new address count value only if the peer is deleted
			if k.peerChangedC != nil {
				k.peerChangedC <- k.conns.Size()
			}
			k.sendNeighbourhoodDepthChange()
		}
	} else {
		k.lconns, _, _, _ = pot.Swap(k.lconns, p, Pof, func(_ pot.Val) pot.Val {
			// v cannot be nil, but no need to check
			return nil
		})

	}


}

func (k *Kademlia) ListKnown() []*BzzAddr {
	res := []*BzzAddr{}

	k.addrs.Each(func(val pot.Val) bool {
		e := val.(*entry)
		res = append(res, e.BzzAddr)
		return true
	})

	return res
}

// EachConn is an iterator with args (base, po, f) applies f to each live peer
// that has proximity order po or less as measured from the base
// if base is nil, kademlia base address is used
func (k *Kademlia) EachConn(base []byte, o int, f func(*Peer, int) bool) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	k.eachConn(base, o, f)
}

func (k *Kademlia) eachConn(base []byte, o int, f func(*Peer, int) bool) {
	if len(base) == 0 {
		base = k.base
	}
	k.conns.EachNeighbour(base, Pof, func(val pot.Val, po int) bool {
		if po > o {
			return true
		}
		return f(val.(*Peer), po)
	})
}

// EachAddr called with (base, po, f) is an iterator applying f to each known peer
// that has proximity order o or less as measured from the base
// if base is nil, kademlia base address is used
func (k *Kademlia) EachAddr(base []byte, o int, f func(*BzzAddr, int) bool) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	k.eachAddr(base, o, f)
}

func (k *Kademlia) eachAddr(base []byte, o int, f func(*BzzAddr, int) bool) {
	if len(base) == 0 {
		base = k.base
	}
	k.addrs.EachNeighbour(base, Pof, func(val pot.Val, po int) bool {
		if po > o {
			return true
		}
		return f(val.(*entry).BzzAddr, po)
	})
}

// NeighbourhoodDepth returns the depth for the pot, see depthForPot
func (k *Kademlia) NeighbourhoodDepth() (depth int) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	return depthForPot(k.conns, k.NeighbourhoodSize, k.base)
}

// neighbourhoodRadiusForPot returns the neighbourhood radius of the kademlia
// neighbourhood radius encloses the nearest neighbour set with size >= neighbourhoodSize
// i.e., neighbourhood radius is the deepest PO such that all bins not shallower altogether
// contain at least neighbourhoodSize connected peers
// if there is altogether less than neighbourhoodSize peers connected, it returns 0
// caller must hold the lock
/*
	neighbourhoodRadiusForPot 检查Pot，根据pivotAddr从近到远，检查每个桶，找到第一个桶里的size < neighbourhoodSize的值
 */
func neighbourhoodRadiusForPot(p *pot.Pot, neighbourhoodSize int, pivotAddr []byte) (depth int) {
	if p.Size() <= neighbourhoodSize {
		return 0
	}
	// total number of peers in iteration
	var size int
	f := func(v pot.Val, i int) bool {
		// po == 256 means that addr is the pivot address(self)
		if i == 256 {
			return true
		}
		size++

		// this means we have all nn-peers.
		// depth is by default set to the bin of the farthest nn-peer
		if size == neighbourhoodSize {
			depth = i
			return false
		}

		return true
	}
	p.EachNeighbour(pivotAddr, Pof, f)
	return depth
}

// depthForPot returns the depth for the pot
// depth is the radius of the minimal extension of nearest neighbourhood that
// includes all empty PO bins. I.e., depth is the deepest PO such that
// - it is not deeper than neighbourhood radius
// - all bins shallower than depth are not empty
// caller must hold the lock
func depthForPot(p *pot.Pot, neighbourhoodSize int, pivotAddr []byte) (depth int) {
	if p.Size() <= neighbourhoodSize {
		return 0
	}
	// determining the depth is a two-step process
	// first we find the proximity bin of the shallowest of the neighbourhoodSize peers
	// the numeric value of depth cannot be higher than this
	maxDepth := neighbourhoodRadiusForPot(p, neighbourhoodSize, pivotAddr)

	// the second step is to test for empty bins in order from shallowest to deepest
	// if an empty bin is found, this will be the actual depth
	// we stop iterating if we hit the maxDepth determined in the first step
	p.EachBin(pivotAddr, Pof, 0, func(po int, _ int, f func(func(pot.Val) bool) bool) bool {
		if po == depth {
			if maxDepth == depth {
				return false
			}
			depth++
			return true
		}
		return false
	})

	return depth
}

// callable decides if an address entry represents a callable peer
func (k *Kademlia) callable(e *entry) bool {
	// not callable if peer is live or exceeded maxRetries
	if e.conn != nil && time.Since(e.connectedAt) > 2* time.Minute && e.retries != 0{
		e.retries = 0
		k.ClearDelay(e.ID(),false)
	}
	if e.conn != nil || e.retries > k.MaxRetries {
		//log.Info(fmt.Sprintf("%08x:connected %v %v", k.BaseAddr()[:4], e, e.retries))

		return false
	}

	if time.Since(e.lastRetry) < 10*time.Second {
		return false
	}
	// calculate the allowed number of retries based on time lapsed since last seen
	timeElasped := int64(time.Since(e.seenAt))/int64(time.Millisecond)
	//指数级增长，直到30分钟一次
	div := int64(e.retries)
	if div > 30{
		div = 10
	}else if div > 20 {
		div = 8
	}else  if div > 10 {
		div = 5
	}
	 shouldWait := int64(0)
	if e.retries > 0 {
		shouldWait =  int64(time.Second) * int64(math.Pow(float64(k.RetryExponent),float64(div))) /int64(time.Millisecond)
	}


	// this is never called concurrently, so safe to increment
	// peer can be retried again
	if timeElasped < shouldWait{
		//log.Trace(fmt.Sprintf("%08x: %v long time since last try (at %v) needed before retry %v, should wait for %v", k.BaseAddr()[:4], e, timeElasped, e.retries, shouldWait))
		return false
	}
	// function to sanction or prevent suggesting a peer
	if k.Reachable != nil && !k.Reachable(e.BzzAddr) {
		log.Trace(fmt.Sprintf("%08x: peer %v is temporarily not callable", k.BaseAddr()[:4], e))
		return false
	}
	e.retries++
	e.seenAt = time.Now()
	e.lastRetry = time.Now()
	log.Trace(fmt.Sprintf("%08x: peer %v is callable", k.BaseAddr()[:4], e))

	return true
}

// BaseAddr return the kademlia base address
func (k *Kademlia) BaseAddr() []byte {
	return k.base
}

// String returns kademlia table + kaddb table displayed with ascii
func (k *Kademlia) String() string {
	k.lock.RLock()
	defer k.lock.RUnlock()
	return k.string()
}

// string returns kademlia table + kaddb table displayed with ascii
// caller must hold the lock
func (k *Kademlia) string() string {
	wsrow := "                          "
	var rows []string

	rows = append(rows, "=========================================================================")
	if len(sv.GitCommit) > 0 {
		rows = append(rows, fmt.Sprintf("commit hash: %s", sv.GitCommit))
	}
	rows = append(rows, fmt.Sprintf("%v KΛÐΞMLIΛ hive: queen's address: %x", time.Now().UTC().Format(time.UnixDate), k.BaseAddr()[:3]))
	rows = append(rows, fmt.Sprintf("population: %d,%d (%d), NeighbourhoodSize: %d, MinBinSize: %d, MaxBinSize: %d", k.conns.Size(),  k.lconns.Size(),k.addrs.Size(), k.NeighbourhoodSize, k.MinBinSize, k.MaxBinSize))

	liverows := make([]string, k.MaxProxDisplay)
	peersrows := make([]string, k.MaxProxDisplay)
	lightrows := make([]string, k.MaxProxDisplay)
	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
	rest := k.conns.Size()
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		var rowlen int
		if po >= k.MaxProxDisplay {
			po = k.MaxProxDisplay - 1
		}
		row := []string{fmt.Sprintf("%2d", size)}
		rest -= size
		f(func(val pot.Val) bool {
			e := val.(*Peer)
			row = append(row, fmt.Sprintf("%x", e.Address()[:2]))
			rowlen++
			return rowlen < 4
		})
		r := strings.Join(row, " ")
		r = r + wsrow
		liverows[po] = r[:31]
		return true
	})

	k.addrs.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		var rowlen int
		if po >= k.MaxProxDisplay {
			po = k.MaxProxDisplay - 1
		}
		if size < 0 {
			panic("wtf")
		}
		row := []string{fmt.Sprintf("%2d", size)}
		// we are displaying live peers too
		f(func(val pot.Val) bool {
			e := val.(*entry)
			row = append(row, Label(e))
			rowlen++
			return rowlen < 4
		})
		r := strings.Join(row, " ")
		r = r + wsrow
		peersrows[po] = r[:31]
		return true
	})
	rest = k.lconns.Size()
	k.lconns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		var rowlen int
		if po >= k.MaxProxDisplay {
			po = k.MaxProxDisplay - 1
		}
		row := []string{fmt.Sprintf("%2d", size)}
		rest -= size
		f(func(val pot.Val) bool {
			e := val.(*Peer)
			row = append(row, fmt.Sprintf("%x", e.Address()[:2]))
			rowlen++
			return rowlen < 4
		})
		r := strings.Join(row, " ")
		r = r + wsrow
		lightrows[po] = strings.Join(row, " ")
		return true
	})

	for i := 0; i < k.MaxProxDisplay; i++ {
		if i == depth {
			rows = append(rows, fmt.Sprintf("============ DEPTH: %d ==========================================", i))
		}
		if uint8(i) == k.depth {
			rows = append(rows, fmt.Sprintf("============ Saturation Depth: %d ==========================================", i))
		}
		left := liverows[i]
		right := peersrows[i]
		third := lightrows[i]
		if len(left) == 0 {
			left = " 0                             "
		}
		if len(right) == 0 {
			right = " 0                             "
		}

		if len(third) == 0 {
			third = " 0"
		}

		rows = append(rows, fmt.Sprintf("%03d %v | %v | %v", i, left, right,third))
	}
	rows = append(rows, "=========================================================================")
	return "\n" + strings.Join(rows, "\n")
}

// PeerPot keeps info about expected nearest neighbours
// used for testing only
// TODO move to separate testing tools file
type PeerPot struct {
	NNSet       [][]byte
	PeersPerBin []int
}

// NewPeerPotMap creates a map of pot record of *BzzAddr with keys
// as hexadecimal representations of the address.
// the NeighbourhoodSize of the passed kademlia is used
// used for testing only
// TODO move to separate testing tools file
func NewPeerPotMap(neighbourhoodSize int, addrs [][]byte) map[string]*PeerPot {

	// create a table of all nodes for health check
	np := pot.NewPot(nil, 0)
	for _, addr := range addrs {
		np, _, _ = pot.Add(np, addr, Pof)
	}
	ppmap := make(map[string]*PeerPot)

	// generate an allknowing source of truth for connections
	// for every kademlia passed
	for i, a := range addrs {

		// actual kademlia depth
		depth := depthForPot(np, neighbourhoodSize, a)

		// all nn-peers
		var nns [][]byte
		peersPerBin := make([]int, depth)

		// iterate through the neighbours, going from the deepest to the shallowest
		np.EachNeighbour(a, Pof, func(val pot.Val, po int) bool {
			addr := val.([]byte)
			// po == 256 means that addr is the pivot address(self)
			// we do not include self in the map
			if po == 256 {
				return true
			}
			// append any neighbors found
			// a neighbor is any peer in or deeper than the depth
			if po >= depth {
				nns = append(nns, addr)
			} else {
				// for peers < depth, we just count the number in each bin
				// the bin is the index of the slice
				peersPerBin[po]++
			}
			return true
		})

		log.Trace(fmt.Sprintf("%x PeerPotMap NNS: %s, peersPerBin", addrs[i][:4], LogAddrs(nns)))
		ppmap[common.Bytes2Hex(a)] = &PeerPot{
			NNSet:       nns,
			PeersPerBin: peersPerBin,
		}
	}
	return ppmap
}

// saturation returns the smallest po value in which the node has less than MinBinSize peers
// if the iterator reaches neighbourhood radius, then the last bin + 1 is returned
func (k *Kademlia) Saturation() int {
	prev := -1
	radius := neighbourhoodRadiusForPot(k.conns, k.NeighbourhoodSize, k.base)
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		prev++
		if po >= radius {
			return false
		}
		return prev == po && size >= k.MinBinSize
	})
	if prev < 0 {
		return 0
	}
	return prev
}

// isSaturated returns true if the kademlia is considered saturated, or false if not.
// It checks this by checking an array of ints called unsaturatedBins; each item in that array corresponds
// to the bin which is unsaturated (number of connections < k.MinBinSize).
// The bin is considered unsaturated only if there are actual peers in that PeerPot's bin (peersPerBin)
// (if there is no peer for a given bin, then no connection could ever be established;
// in a God's view this is relevant as no more peers will ever appear on that bin)
func (k *Kademlia) isSaturated(peersPerBin []int, depth int) bool {
	// depth could be calculated from k but as this is called from `GetHealthInfo()`,
	// the depth has already been calculated so we can require it as a parameter

	// early check for depth
	if depth != len(peersPerBin) {
		return false
	}
	unsaturatedBins := make([]int, 0)
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {

		if po >= depth {
			return false
		}
		log.Trace("peers per bin", "peersPerBin[po]", peersPerBin[po], "po", po)
		// if there are actually peers in the PeerPot who can fulfill k.MinBinSize
		if size < k.MinBinSize && size < peersPerBin[po] {
			log.Trace("connections for po", "po", po, "size", size)
			unsaturatedBins = append(unsaturatedBins, po)
		}
		return true
	})

	log.Trace("list of unsaturated bins", "unsaturatedBins", unsaturatedBins)
	return len(unsaturatedBins) == 0
}

// knowNeighbours tests if all neighbours in the peerpot
// are found among the peers known to the kademlia
// It is used in Healthy function for testing only
// TODO move to separate testing tools file
func (k *Kademlia) knowNeighbours(addrs [][]byte) (got bool, n int, missing [][]byte) {
	pm := make(map[string]bool)
	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
	// create a map with all peers at depth and deeper known in the kademlia
	k.eachAddr(nil, 255, func(p *BzzAddr, po int) bool {
		// in order deepest to shallowest compared to the kademlia base address
		// all bins (except self) are included (0 <= bin <= 255)
		if po < depth {
			return false
		}
		pk := common.Bytes2Hex(p.Address())
		pm[pk] = true
		return true
	})

	// iterate through nearest neighbors in the peerpot map
	// if we can't find the neighbor in the map we created above
	// then we don't know all our neighbors
	// (which sadly is all too common in modern society)
	var gots int
	var culprits [][]byte
	for _, p := range addrs {
		pk := common.Bytes2Hex(p)
		if pm[pk] {
			gots++
		} else {
			log.Trace(fmt.Sprintf("%08x: known nearest neighbour %s not found", k.base, pk))
			culprits = append(culprits, p)
		}
	}
	return gots == len(addrs), gots, culprits
}
func (k *Kademlia)GetConnectedNodes() (int,int){
	return k.conns.Size(),k.lconns.Size()
}
// connectedNeighbours tests if all neighbours in the peerpot
// are currently connected in the kademlia
// It is used in Healthy function for testing only
func (k *Kademlia) connectedNeighbours(peers [][]byte) (got bool, n int, missing [][]byte) {
	pm := make(map[string]bool)

	// create a map with all peers at depth and deeper that are connected in the kademlia
	// in order deepest to shallowest compared to the kademlia base address
	// all bins (except self) are included (0 <= bin <= 255)
	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
	k.eachConn(nil, 255, func(p *Peer, po int) bool {
		if po < depth {
			return false
		}
		pk := common.Bytes2Hex(p.Address())
		pm[pk] = true
		return true
	})

	// iterate through nearest neighbors in the peerpot map
	// if we can't find the neighbor in the map we created above
	// then we don't know all our neighbors
	var gots int
	var culprits [][]byte
	for _, p := range peers {
		pk := common.Bytes2Hex(p)
		if pm[pk] {
			gots++
		} else {
			log.Trace(fmt.Sprintf("%08x: ExpNN: %s not found", k.base, pk))
			culprits = append(culprits, p)
		}
	}
	return gots == len(peers), gots, culprits
}

// Health state of the Kademlia
// used for testing only
type Health struct {
	KnowNN           bool     // whether node knows all its neighbours
	CountKnowNN      int      // amount of neighbors known
	MissingKnowNN    [][]byte // which neighbours we should have known but we don't
	ConnectNN        bool     // whether node is connected to all its neighbours
	CountConnectNN   int      // amount of neighbours connected to
	MissingConnectNN [][]byte // which neighbours we should have been connected to but we're not
	// Saturated: if in all bins < depth number of connections >= MinBinsize or,
	// if number of connections < MinBinSize, to the number of available peers in that bin
	Saturated bool
	Hive      string
}

// GetHealthInfo reports the health state of the kademlia connectivity
//
// The PeerPot argument provides an all-knowing view of the network
// The resulting Health object is a result of comparisons between
// what is the actual composition of the kademlia in question (the receiver), and
// what SHOULD it have been when we take all we know about the network into consideration.
//
// used for testing only
func (k *Kademlia) GetHealthInfo(pp *PeerPot) *Health {
	k.lock.RLock()
	defer k.lock.RUnlock()
	if len(pp.NNSet) < k.NeighbourhoodSize {
		log.Warn("peerpot NNSet < NeighbourhoodSize")
	}
	gotnn, countgotnn, culpritsgotnn := k.connectedNeighbours(pp.NNSet)
	knownn, countknownn, culpritsknownn := k.knowNeighbours(pp.NNSet)
	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)

	// check saturation
	saturated := k.isSaturated(pp.PeersPerBin, depth)

	log.Trace(fmt.Sprintf("%08x: healthy: knowNNs: %v, gotNNs: %v, saturated: %v\n", k.base, knownn, gotnn, saturated))
	return &Health{
		KnowNN:           knownn,
		CountKnowNN:      countknownn,
		MissingKnowNN:    culpritsknownn,
		ConnectNN:        gotnn,
		CountConnectNN:   countgotnn,
		MissingConnectNN: culpritsgotnn,
		Saturated:        saturated,
		Hive:             k.string(),
	}
}

// Healthy return the strict interpretation of `Healthy` given a `Health` struct
// definition of strict health: all conditions must be true:
// - we at least know one peer
// - we know all neighbors
// - we are connected to all known neighbors
// - it is saturated
func (h *Health) Healthy() bool {
	return h.KnowNN && h.ConnectNN && h.CountKnowNN > 0 && h.Saturated
}
