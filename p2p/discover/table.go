// Copyright 2015 The go-ethereum Authors
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

// Package discover implements the Node Discovery Protocol.
//
// The Node Discovery protocol provides a way to find RLPx nodes that
// can be connected to. It uses a Kademlia-like protocol to maintain a
// distributed database of the IDs and endpoints of all listening
// nodes.
package discover

import (
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/plotozhu/MDCMainnet/common"
	"github.com/plotozhu/MDCMainnet/crypto"
	"github.com/plotozhu/MDCMainnet/log"
	"github.com/plotozhu/MDCMainnet/p2p/enode"
	"github.com/plotozhu/MDCMainnet/p2p/netutil"
)

const (
	alpha           = 3  // Kademlia concurrency factor
	bucketSize      = 16 // Kademlia bucket size
	maxReplacements = 10 // Size of per-bucket replacement list

	// We keep buckets for the upper 1/15 of distances because
	// it's very unlikely we'll ever encounter a node that's closer.
	hashBits          = len(common.Hash{}) * 8
	nBuckets          = hashBits / 15       // Number of buckets
	bucketMinDistance = hashBits - nBuckets // Log distance of closest bucket

	// IP address limits.
	bucketIPLimit, bucketSubnet = 2, 24 // at most 2 addresses from the same /24
	tableIPLimit, tableSubnet   = 10, 24

	maxFindnodeFailures = 5 // Nodes exceeding this limit are dropped
	refreshInterval     = 10 * time.Second //30 * time.Minute
	revalidateInterval  = 10 * time.Second
	copyNodesInterval   = 30 * time.Second
	seedMinTableTime    = 5 * time.Minute
	seedCount           = 30
	seedMaxAge          = 5 * 24 * time.Hour
)

type Table struct {
	mutex   sync.Mutex        // protects buckets, bucket content, nursery, rand
	buckets [nBuckets]*bucket // index of known nodes by distance
	nursery []*node           // bootstrap nodes
	rand    *mrand.Rand       // source of randomness, periodically reseeded
	ips     netutil.DistinctNetSet

	db         *enode.DB // database of known nodes
	net        transport
	refreshReq chan chan struct{}
	initDone   chan struct{}

	closeOnce sync.Once
	closeReq  chan struct{}
	closed    chan struct{}
	notifyChannel    chan struct{}
	nodeAddedHook func(*node) // for testing
	allNodes     map[enode.ID]*node

}
type AttributeID uint8

const (
	AttrLastSeen  AttributeID = 0
	AttrTestAt    AttributeID = 1
	AttrFindAt    AttributeID = 2
	AttrAddAt    AttributeID = 3
	AttrLatency AttributeID = 4

)
// transport is implemented by the UDP transport.
// it is an interface so we can test without opening lots of UDP
// sockets and without generating a private key.
type transport interface {
	self() *enode.Node
	ping(enode.ID, *net.UDPAddr) (error, time.Duration)
	findnode(toid enode.ID, addr *net.UDPAddr, target encPubkey) ([]*node, error)
	close()
}
type NodeQueue struct {
	entries  []*node
	exists   map[enode.ID]*node
	maxsize int
	mutex   sync.Mutex
}

func NewNodeQueue(maxsize int) *NodeQueue{
	return &NodeQueue{
		entries:make([]*node,0),
		exists:make(map[enode.ID]*node),
		maxsize:maxsize,
	}
}
type Attribute struct {
	attrId AttributeID
	attr interface{}
}
type Attributes []*Attribute
func (nq *NodeQueue)loadNodes(nodes []*node) {
	nq.mutex.Lock()

	nq.entries=make([]*node,0)
	nq.exists=make(map[enode.ID]*node)
	nq.mutex.Unlock()
	for _,anode := range nodes {
		nq.AddNode(anode,false)
	}
}

func (nq *NodeQueue)getNodeByIndex(index int) *node{
	nq.mutex.Lock()

	nq.mutex.Unlock()
	if len(nq.entries) > index {
		return nq.entries[index]
	}
	return nil
}

func (nq *NodeQueue)hasDuplicated(nodeId enode.ID) bool {
	nq.mutex.Lock()

	defer nq.mutex.Unlock()

	dup := 0
	for _,node := range nq.entries {
		if node.ID() == nodeId {
			dup++
		}
	}
	return dup > 1
}

func (nq *NodeQueue)ReplaceNode(anode *node, checkReplace func(nodeInEntries *node) bool) bool {
    //log.Info("1")
	//defer func(){log.Info("1.1")}()
	for n,oldNode := range nq.entries {

		if checkReplace(oldNode)  {
			nq.entries[n] = anode
			delete(nq.exists,oldNode.ID())
			nq.exists[anode.ID()] = anode

			return  true
		}
	}
	return false

}
func (nq *NodeQueue)UpdateAttriutes(nodeId enode.ID, attrs Attributes) bool{
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
    //log.Info("2")
	//defer func(){log.Info("2.1")}()
	_,ok := nq.exists[nodeId]
	if(!ok){
		return false
	}
	for _,anode := range nq.entries {
		if anode.ID() == nodeId {
			for _,attr := range attrs {
				switch attr.attrId{
				case AttrAddAt:
					anode.addedAt = attr.attr.(time.Time)
				case AttrFindAt:
					anode.findAt = attr.attr.(time.Time)
					if anode.findAt.UnixNano() == 0 {
						log.Trace("peer without find: ","id",anode.ID(),"ip",anode.IP(),"port",anode.UDP())
					}
				case AttrTestAt:
					anode.testAt = attr.attr.(time.Time)
				case AttrLatency:
					anode.latency = attr.attr.(int64)
				}
			}
			return true
		}
	}
	return  false
}
func (nq *NodeQueue)AddNode(anode *node,shouldUpdate bool) bool{
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
    //log.Info("3")
	//defer func(){log.Info("3.1")}()
	_,ok := nq.exists[anode.ID()]
	if ok {
		if shouldUpdate {
			for n,node := range nq.entries {
				if node.ID() == anode.ID() {
					nq.entries[n] = anode
					nq.exists[anode.ID()] = anode
					if anode.findAt == time.Unix (0,0) {
						log.Trace("peer without find: ","id",anode.ID(),"ip",anode.IP(),"port",anode.UDP())
					}
					return true
				}
			}
			log.Error("Error in nq.entries","nodeId",anode.ID(),"reason","node exist in exists,but not in entries")
			delete(nq.exists,anode.ID())
			return false //update failed, should not occur
		}else{
			if anode.findAt== time.Unix (0,0) {
				log.Trace("peer without find: ","id",anode.ID(),"ip",anode.IP(),"port",anode.UDP())
			}
		}
		return true
	}else {
		if len(nq.entries) >= nq.maxsize {
			return false
		}else {
			nq.entries = append(nq.entries,anode)
			nq.exists[anode.ID()] = anode

			if  anode.findAt == time.Unix (0,0) {
				log.Trace("peer without find: ","id",anode.ID(),"ip",anode.IP(),"port",anode.UDP())
			}
			return  true
		}
	}
}
func (nq *NodeQueue)GetEntries() []*node{
   // log.Info("4")
	//defer func(){log.Info("4.1")}()
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
	result := make([]*node,len(nq.entries))
	for i := 0; i < len(nq.entries); i++ {
		result[i] = nq.entries[i]
	}
	return result
}
func (nq *NodeQueue)RemoveNode(nodeId enode.ID) (bool,*node){
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
    //log.Info("5")
	//defer func(){log.Info("5.1")}()
	_,ok := nq.exists[nodeId]
	if ok {
		delete (nq.exists,nodeId)
		for n,node := range nq.entries {
			if node.ID() == nodeId {
				nq.entries = append(nq.entries[:n],nq.entries[n+1:]...)
				return true,node
			}
		}
		return false,nil

	}else {
		return false,nil
	}
}
func (nq *NodeQueue)Contains(nodeId enode.ID) bool {
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
    //log.Info("6")
	//defer func(){log.Info("6.1")}()
	_,ok := nq.exists[nodeId]
	return ok
}
func (nq *NodeQueue)Get(nodeId enode.ID) *node {
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
    //log.Info("6")
	//defer func(){log.Info("6.1")}()
	result,_ := nq.exists[nodeId]
	return result
}
func (nq *NodeQueue)MoveFront(nodeId enode.ID) bool {
    //log.Info("7")
	//defer func(){log.Info("7.1")}()
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
	_,ok := nq.exists[nodeId]
	if !ok {
		return false
	}
	for n,anode := range nq.entries {
		if anode.ID() == nodeId {
			nq.entries = append(nq.entries[:n],nq.entries[n+1:]...)
			nq.entries = append([]*node{anode},nq.entries...)
			return true
		}
	}
	return false
}
func (nq *NodeQueue)MoveBack(nodeId enode.ID) bool {
    //log.Info("8")
	//defer func(){log.Info("8.1")}()
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
	_,ok := nq.exists[nodeId]
	if !ok {
		return false
	}
	for n,anode := range nq.entries {
		if anode.ID() == nodeId {
			nq.entries = append(nq.entries[:n],nq.entries[n+1:]...)
			nq.entries = append(nq.entries,anode)
			return true
		}
	}
	return false
}

func (nq *NodeQueue)Length() int {
   //log.Info("9")
	//defer func(){log.Info("9.1")}()
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
	return len(nq.entries)
}
// bucket contains nodes, ordered by their last activity. the entry
// that was most recently active is the first element in entries.
type bucket struct {
	connects     *NodeQueue
	entries     * NodeQueue // live entries, sorted by time of last contact
	replacements *NodeQueue // recently seen nodes to be used if revalidation fails
	ips          netutil.DistinctNetSet
}

func newTable(t transport, db *enode.DB, bootnodes []*enode.Node) (*Table, error) {
	tab := &Table{
		net:        t,
		db:         db,
		refreshReq: make(chan chan struct{}),
		initDone:   make(chan struct{}),
		closeReq:   make(chan struct{}),
		closed:     make(chan struct{}),
		rand:       mrand.New(mrand.NewSource(0)),
		ips:        netutil.DistinctNetSet{Subnet: tableSubnet, Limit: tableIPLimit},
		allNodes:   make(map[enode.ID]*node),
	}
	if err := tab.setFallbackNodes(bootnodes); err != nil {
		return nil, err
	}
	for i := range tab.buckets {
		tab.buckets[i] = &bucket{
			connects:NewNodeQueue(bucketSize),
			entries:NewNodeQueue(bucketSize),
			replacements:NewNodeQueue(2*bucketSize),
			ips: netutil.DistinctNetSet{Subnet: bucketSubnet, Limit: bucketIPLimit},
		}
	}
	tab.seedRand()
	tab.loadSeedNodes()
	tab.nodeAddedHook = func(i *node) {
		log.Debug("noded added:","id",i.ID(),"addr",i.IP(),"port",i.UDP())
		if tab.notifyChannel != nil {
			tab.notifyChannel <- struct{}{}
		}
		//log.Debug("noded OK:","id",i.ID(),"addr",i.IP(),"port",i.UDP())
	}
	go tab.loop()
	return tab, nil
}

func (tab *Table) self() *enode.Node {
	return tab.net.self()
}

func (tab *Table) seedRand() {
	var b [8]byte
	crand.Read(b[:])

	tab.mutex.Lock()
	tab.rand.Seed(int64(binary.BigEndian.Uint64(b[:])))
	tab.mutex.Unlock()
}

// ReadRandomNodes fills the given slice with random nodes from the table. The results
// are guaranteed to be unique for a single invocation, no node will appear twice.
func (tab *Table) ReadRandomNodes(buf []*enode.Node) (n int) {
    //log.Info("11")
	//defer func(){log.Info("11.1")}()
	if !tab.isInitDone() {
		return 0
	}
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	// Find all non-empty buckets and get a fresh slice of their entries.
	var buckets [][]*node
	for _, b := range &tab.buckets {
		if b.entries.Length() > 0 {
			buckets = append(buckets, b.entries.GetEntries())
		}
	}
	if len(buckets) == 0 {
		return 0
	}
	// Shuffle the buckets.
	for i := len(buckets) - 1; i > 0; i-- {
		j := tab.rand.Intn(len(buckets))
		buckets[i], buckets[j] = buckets[j], buckets[i]
	}
	// Move head of each bucket into buf, removing buckets that become empty.
	var i, j int
	for ; i < len(buf); i, j = i+1, (j+1)%len(buckets) {
		b := buckets[j]
		buf[i] = unwrapNode(b[0])
		buckets[j] = b[1:]
		if len(b) == 1 {
			buckets = append(buckets[:j], buckets[j+1:]...)
		}
		if len(buckets) == 0 {
			break
		}
	}
	return i + 1
}

// Close terminates the network listener and flushes the node database.
func (tab *Table) Close() {
    //log.Info("91")
	//defer func(){log.Info("91.1")}()
//	log.Info("Close Kad Table")
	tab.closeOnce.Do(func() {
		if tab.net != nil {
			tab.net.close()
		}
		// Wait for loop to end.
		close(tab.closeReq)
		<-tab.closed
	})
}

// setFallbackNodes sets the initial points of contact. These nodes
// are used to connect to the network if the table is empty and there
// are no known nodes in the database.
func (tab *Table) setFallbackNodes(nodes []*enode.Node) error {
	for _, n := range nodes {
		if err := n.ValidateComplete(); err != nil {
			return fmt.Errorf("bad bootstrap node %q: %v", n, err)
		}
	}
	tab.nursery = wrapNodes(nodes)
	return nil
}

// isInitDone returns whether the table's initial seeding procedure has completed.
func (tab *Table) isInitDone() bool {
    //log.Info("12")
	//defer func(){log.Info("12.1")}()
	select {
	case <-tab.initDone:
		return true
	default:
		return false
	}
}


type SortableNode []*node

func (c SortableNode) Len() int {
	return len(c)
}
func (c SortableNode) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
func (c SortableNode) Less(i, j int) bool {
	return c[i].latency < c[j].latency
}



//按延时从小到大的顺序排好
func (tab *Table) GetKnownNodesSorted() []*enode.Node{
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
    //log.Info("13")
	//defer func(){log.Info("13.1")}()
	ret := make(SortableNode,0)
	//log.Debug("step 1")
	for _,bucket := range tab.buckets {
		bucketRet := make(SortableNode,0)
		for _,node := range bucket.entries.GetEntries() {
			if !enode.IsLightNode(enode.NodeTypeOption(node.NodeType())) && !enode.IsBootNode(enode.NodeTypeOption(node.NodeType()) ) {
				bucketRet = append(bucketRet,node)
			}

		}
		sort.Sort(bucketRet)
		for i,sortedNode := range bucketRet {
			if i <= 5 || sortedNode.latency < int64(100*time.Millisecond) {
				ret = append(ret,sortedNode)
			}
		}

	}
	//log.Debug("step 2")
	result := make([]*enode.Node,len(ret))
	for i,node := range ret {
		result[i] = unwrapNode(node)
	}
	log.Trace("Known nodes:","count",len(result))
	return result
}
func (tab *Table) OnNodeChanged(nodeChanged chan struct{}){
	//tab.nodeCnhanged
	tab.notifyChannel = nodeChanged
}
// Resolve searches for a specific node with the given ID.
// It returns nil if the node could not be found.
func (tab *Table) Resolve(n *enode.Node) *enode.Node {
	// If the node is present in the local table, no
	// network interaction is required.
    //log.Info("14")
	//defer func(){log.Info("14.1")}()
	hash := n.ID()
	tab.mutex.Lock()
	cl := tab.closest(hash, 1)
	tab.mutex.Unlock()
	if len(cl.entries) > 0 && cl.entries[0].ID() == hash {
		return unwrapNode(cl.entries[0])
	}
	// Otherwise, do a network lookup.
	result := tab.lookup(encodePubkey(n.Pubkey()), true)
	for _, n := range result {
		if n.ID() == hash {
			return unwrapNode(n)
		}
	}
	return nil
}

// LookupRandom finds random nodes in the network.
func (tab *Table) LookupRandom() []*enode.Node {
	var target encPubkey
	crand.Read(target[:])
    //log.Info("15")
	//defer func(){log.Info("15.1")}()
	return unwrapNodes(tab.lookup(target, true))
}

func (tab *Table)ProcessLive(nodeId enode.ID, tm time.Duration,isLive bool) {

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
    //log.Info("16")
	//defer func(){log.Info("16.1")}()
	bucket :=tab.bucket(nodeId)
	tab.execValidateNode(nodeId,bucket,tm,isLive)
}
// lookup performs a network search for nodes close to the given target. It approaches the
// target by querying nodes that are closer to it on each iteration. The given target does
// not need to be an actual node identifier.
func (tab *Table) lookup(targetKey encPubkey, refreshIfEmpty bool) []*node {
	var (
		target         = enode.ID(crypto.Keccak256Hash(targetKey[:]))
		asked          = make(map[enode.ID]bool)
		seen           = make(map[enode.ID]bool)
		reply          = make(chan []*node, alpha)
		pendingQueries = 0
		result         *nodesByDistance
	)
	// don't query further if we hit ourself.
	// unlikely to happen often in practice.
	asked[tab.self().ID()] = true
    //log.Info("17")
	//defer func(){log.Info("17.1")}()
	for {
		tab.mutex.Lock()
		// generate initial result set
		result = tab.closest(target, bucketSize)
		tab.mutex.Unlock()
		if len(result.entries) > 0 || !refreshIfEmpty {
			break
		}
		// The result set is empty, all nodes were dropped, refresh.
		// We actually wait for the refresh to complete here. The very
		// first query will hit this case and run the bootstrapping
		// logic.
		<-tab.refresh()
		refreshIfEmpty = false
	}

	for {
		// ask the alpha closest nodes that we haven't asked yet
		for i := 0; i < len(result.entries) && pendingQueries < alpha; i++ {
			n := result.entries[i]
			nodeType := enode.NodeTypeOption(n.NodeType())
			if !asked[n.ID()] && ( !enode.IsLightNode(nodeType) ) && time.Since(n.findAt)> 10*time.Second{
				asked[n.ID()] = true
				pendingQueries++
    			log.Trace("Find node:","id",n.ID(),"lastFind:",n.findAt,"new time:",time.Now())
				n.findAt = time.Now()
				
				go tab.findnode(n, targetKey, reply)
			}
		}
		if pendingQueries == 0 {
			// we have asked all closest nodes, stop the search
			break
		}
		select {
		case nodes := <-reply:
			for _, n := range nodes {
				if n != nil && !seen[n.ID()] {
					seen[n.ID()] = true
					result.push(n, bucketSize)
				}
			}
		case <-tab.closeReq:
			return nil // shutdown, no need to continue.
		}
		pendingQueries--
	}
	return result.entries
}

func (tab *Table) findnode(n *node, targetKey encPubkey, reply chan<- []*node) {
    //log.Info("18")
	//defer func(){log.Info("18.1")}()

	fails := tab.db.FindFails(n.ID(), n.IP())
	r, err := tab.net.findnode(n.ID(), n.addr(), targetKey)
	if err == errClosed {
		// Avoid recording failures on shutdown.
		reply <- nil
		return
	} else if err != nil || len(r) == 0 {
		fails++
		tab.db.UpdateFindFails(n.ID(), n.IP(), fails)
		log.Trace("Findnode failed", "id", n.ID(), "failcount", fails, "err", err)
		//by Aegon findnode在返回达不到16个的时候，就认为是错的，然后几次错误后就把这个节点给
		/*if fails >= maxFindnodeFailures {
    //log.Info("Too many findnode failures, dropping", "id", n.ID(), "failcount", fails)
			tab.delete(n)
		}*/
	} else if fails > 0 {
		tab.db.UpdateFindFails(n.ID(), n.IP(), fails-1)
	}

	// Grab as many nodes as possible. Some of them might not be alive anymore, but we'll
	// just remove those again during revalidation.
	for _, n := range r {
		tab.addSeenNode(n)
	}
	reply <- r

}

func (tab *Table) setupRefreshTime() time.Duration{
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
    //log.Info("19")
	//defer func(){log.Info("19.1")}()
	knownCount := 0
	for _,bucket := range tab.buckets {
		knownCount += bucket.connects.Length()
		knownCount += bucket.entries.Length()
	}
	if knownCount < 5 {
		return 2 * time.Second
	}else if  knownCount <= 100  {
		return time.Duration(10 + 10*knownCount) * time.Second
	}else {
		return 30*time.Minute
	}
}
func (tab *Table) refresh() <-chan struct{} {
    //log.Info("20")
	//defer func(){log.Info("20.1")}()
	done := make(chan struct{})
	select {
	case tab.refreshReq <- done:
	case <-tab.closeReq:
		close(done)
	}
	return done
}

// loop schedules refresh, revalidate runs and coordinates shutdown.
func (tab *Table) loop() {
	var (
		revalidate     = time.NewTimer(tab.nextRevalidateTime())
		refresh        = time.NewTimer(refreshInterval)
		replace        = time.NewTicker(tab.nextRevalidateTime())
		copyNodes      = time.NewTicker(copyNodesInterval)
		refreshDone    = make(chan struct{})           // where doRefresh reports completion
		revalidateDone chan struct{}                   // where doRevalidate reports completion
		replaceDone    chan struct{}
		waiting        = []chan struct{}{tab.initDone} // holds waiting callers while doRefresh runs
	)
	defer refresh.Stop()
	defer revalidate.Stop()
	defer copyNodes.Stop()
	defer replace.Stop()
	// Start initial refresh.
	go func (){
		tab.doRefresh(refreshDone)
		revalidateDone = make(chan struct{})
		go tab.doReplacementCheck(revalidateDone)
		select {
			case <- revalidateDone:
		case <-tab.closeReq:
		}
	}()

loop:
	for {
		select {
		case <-refresh.C:
			tab.seedRand()
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				go tab.doRefresh(refreshDone)
			}
		case req := <-tab.refreshReq:
			waiting = append(waiting, req)
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				go tab.doRefresh(refreshDone)
			}
		case <-refreshDone:
			for _, ch := range waiting {
				close(ch)
			}
			waiting, refreshDone = nil, nil
			refresh.Reset(tab.setupRefreshTime())
		case <-revalidate.C:
			revalidateDone = make(chan struct{})
			go tab.doRevalidate(revalidateDone)
		case <-revalidateDone:
			revalidate.Reset(tab.nextRevalidateTime())
			revalidateDone = nil
		case <-replace.C:
			replaceDone = make(chan struct{})
			go tab.doReplacementCheck(replaceDone)
		case <-replaceDone:
			revalidate.Reset(tab.nextRevalidateTime())
			replaceDone = nil

		case <-copyNodes.C:
			go tab.copyLiveNodes()
		case <-tab.closeReq:
			break loop
		}
	}

	if refreshDone != nil {
		<-refreshDone
	}
	for _, ch := range waiting {
		close(ch)
	}
	if revalidateDone != nil {
		<-revalidateDone
	}

	if refreshDone != nil {
		<-refreshDone
	}
	if replaceDone != nil {
		<- replaceDone
	}
    //log.Info("Do tab finished")
	close(tab.closed)
}

// doRefresh performs a lookup for a random target to keep buckets
// full. seed nodes are inserted if the table is empty (initial
// bootstrap or discarded faulty peers).
func (tab *Table) doRefresh(done chan struct{}) {
    //log.Info("31")
	//defer func(){log.Info("31.1")}()
	defer close(done)

	// Load nodes from the database and insert
	// them. This should yield a few previously seen nodes that are
	// (hopefully) still alive.
	tab.loadSeedNodes()

	// Run self lookup to discover new neighbor nodes.
	// We can only do this if we have a secp256k1 identity.
	var key ecdsa.PublicKey
	if err := tab.self().Load((*enode.Secp256k1)(&key)); err == nil {
		tab.lookup(encodePubkey(&key), false)
	}

	// The Kademlia paper specifies that the bucket refresh should
	// perform a lookup in the least recently used bucket. We cannot
	// adhere to this because the findnode target is a 512bit value
	// (not hash-sized) and it is not easily possible to generate a
	// sha3 preimage that falls into a chosen bucket.
	// We perform a few lookups with a random target instead.
	for i := 0; i < 3; i++ {
		var target encPubkey
		crand.Read(target[:])
		tab.lookup(target, false)
	}
}

func (tab *Table) loadSeedNodes() {
    //log.Info("32")
	//defer func(){log.Info("32.1")}()
	seeds := wrapNodes(tab.db.QuerySeeds(seedCount, seedMaxAge))
	sortNodes := SortableNode(seeds)
	for _,node := range sortNodes {
		node.latency = tab.db.GetNodeLatency(node.ID(),node.IP())
	}
	sort.Sort(sortNodes)
	seeds = append(sortNodes, tab.nursery...)
	for i := range seeds {
		seed := seeds[i]
		seed.latency =tab.db.GetNodeLatency(seed.ID(),seed.IP())
		age := log.Lazy{Fn: func() interface{} { return time.Since(tab.db.LastPongReceived(seed.ID(), seed.IP())) }}
		log.Trace("Found seed node in database", "id", seed.ID(), "addr", seed.addr(), "age", age,"latency",seed.latency)
		tab.addSeenNode(seed)
	}
}
func (tab *Table)execValidateNode(nodeId enode.ID,b *bucket,tm time.Duration,alive bool ){
    //log.Info("33")
	//defer func(){log.Info("33.1")}()
	//b := tab.buckets[bi]
	var anode  *node
	anode = nil
	//state 0 not found /1 in  connects /2 in entries /3 in replacement
	state := 0
	if anode = b.connects.Get(nodeId) ; anode != nil {
		state = 1
	} else if anode = b.entries.Get(nodeId); anode != nil {
		state = 2
	} else if anode = b.replacements.Get(nodeId); anode != nil {
		state = 3
	}


	if alive {
		if state == 2 || state == 3 {
			anode.latency = int64(tm)
			anode.testAt = time.Now()
			addToEntriesOK := false
    		log.Trace("Node updated","node ID",nodeId,"addr",anode.IP().String(),"port",anode.UDP(),"latency",anode.latency,"findAt",anode.findAt,"state",state)
		 		//在entries中
		 		if state == 2 {
					//移动到entries的最前面
					b.entries.MoveFront(anode.ID())
					b.entries.UpdateAttriutes(anode.ID(),Attributes{&Attribute{AttrLatency,(int64(tm))},&Attribute{AttrTestAt,time.Now()}})

					addToEntriesOK = true

				}else {
					addToEntriesOK = b.entries.AddNode(anode,true)
					if tab.nodeAddedHook !=  nil {
						tab.nodeAddedHook(anode)
					}
				}

				if addToEntriesOK{
					b.replacements.RemoveNode(anode.ID())
				}else {


					if !b.replacements.AddNode(anode,true) {
						// 节点在replacement中，测试通过了，但是entries队列满了，这个节点应该放在哪里？
						//测试失败的，我们可以考虑移动替换一个测试没有通过的
						b.replacements.ReplaceNode(anode,func(oldNode *node) bool{
							return oldNode.latency >= int64(1*time.Hour)
						})
					}
				}

		}	else if  state == 0 {
				log.Error("Error node ping????","node ID",nodeId)
		} else {
			//节点在已经连接队列中，保险起见，从entries和 replacment中删除
			b.replacements.RemoveNode(anode.ID())
			b.entries.RemoveNode(anode.ID())

		}
	} else {
    //log.Info("Node failed","node ID",nodeId,"addr",anode.IP().String(),"port",anode.UDP(),"latency",anode.latency,"findAt",anode.findAt,"state",state)

		if state != 0 {
			anode.testAt = time.Now()
			anode.latency = int64(100*time.Hour)
			b.connects.RemoveNode(anode.ID()) //从connect中移除（假如有）
			b.entries.RemoveNode(anode.ID()) //从entries中移除（假如有）
			//保障删除
			b.replacements.AddNode(anode,true)
			//断开的连接移动到最后
			b.replacements.MoveBack(anode.ID()) //从replacement中移除（假如有）

		} else {
			log.Error("Error node ping????","node ID",nodeId)
		}

	}

}

// doRevalidate checks that the last node in a random bucket is still live
// and replaces or deletes the node if it isn't.
func (tab *Table) doReplacementCheck(done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
    //log.Info("34")
	//defer func(){log.Info("34.1")}()
	last, _ := tab.replaceNodeToCheck()
	if last == nil {
		// No non-empty bucket found.
		return
	}

	// Ping the selected node and wait for a pong.
	 tab.net.ping(last.ID(), last.addr())


}

// doRevalidate checks that the last node in a random bucket is still live
// and replaces or deletes the node if it isn't.
func (tab *Table) doRevalidate(done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
    //log.Info("35")
	//defer func(){log.Info("35.1")}()
	//all connected is set to seen now
	for _,bucket := range tab.buckets {
		for _,node := range bucket.connects.GetEntries() {

			tab.db.UpdateLastPongReceived(node.ID(), node.IP(), time.Now())
		}
	}
	last, _ := tab.nodeToRevalidate()
	if last == nil {
		// No non-empty bucket found.
		return
	}

	// Ping the selected node and wait for a pong. it will use processLive来进行结果处理
	tab.net.ping(last.ID(), last.addr())


}

// nodeToRevalidate returns the last node in a random, non-empty bucket.
func (tab *Table) nodeToRevalidate() (n *node, bi int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
    //log.Info("36")
	//defer func(){log.Info("36.1")}()
	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if b.entries.Length() > 0 {
			last := b.entries.GetEntries()[b.entries.Length()-1]
			if time.Since(last.testAt) > 30 *time.Second {
				return last, bi
			}else{
				return nil,0
			}
		}
	}
	return nil, 0
}
// nodeToRevalidate returns the last node in a random, non-empty bucket.
func (tab *Table) replaceNodeToCheck() (n *node, bi int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
    //log.Info("37")
	//defer func(){log.Info("37.1")}()
	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if b.entries.Length() + b.connects.Length() < bucketSize && b.replacements.Length() > 0 {
			last := b.replacements.GetEntries()[0]
			if time.Since(last.testAt) > 30 *time.Second {
				return last, bi
			}else{
				return nil,0
			}

		}
	}
	return nil, 0
}
func (tab *Table) nextRevalidateTime() time.Duration {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
    //log.Info("38")
	//defer func(){log.Info("38.1")}()
	return time.Duration(tab.rand.Int63n(int64(revalidateInterval)))
}

// copyLiveNodes adds nodes from the table to the database if they have been in the table
// longer then minTableTime.
func (tab *Table) copyLiveNodes() {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
    //log.Info("39")
	//defer func(){log.Info("39.1")}()
	now := time.Now()
	for _, b := range &tab.buckets {
		for _, n := range b.entries.GetEntries() {
			if now.Sub(n.addedAt) >= seedMinTableTime {
				tab.db.UpdateNode(unwrapNode(n))
			}
		}
		for _, n := range b.connects.GetEntries() {
			tab.db.UpdateNode(unwrapNode(n))
		}
	}
}

// closest returns the n nodes in the table that are closest to the
// given id. The caller must hold tab.mutex.
func (tab *Table) closest(target enode.ID, nresults int) *nodesByDistance {
	// This is a very wasteful way to find the closest nodes but
	// obviously correct. I believe that tree-based buckets would make
	// this easier to implement efficiently.
    //log.Info("40")
	//defer func(){log.Info("40.1")}()
	close := &nodesByDistance{target: target}
	for _, b := range &tab.buckets {
		for _, n := range b.connects.GetEntries() {
			close.push(n, nresults)
		}
		for _, n := range b.entries.GetEntries() {
			close.push(n, nresults)
		}
	}
	return close
}

func (tab *Table) len() (n int) {
	for _, b := range &tab.buckets {
		n += len(b.entries.GetEntries())
	}
	return n
}

// bucket returns the bucket for the given node ID hash.
func (tab *Table) bucket(id enode.ID) *bucket {
    //log.Info("41")
	//defer func(){log.Info("41.1")}()
	d := enode.LogDist(tab.self().ID(), id)
	if d <= bucketMinDistance {
		return tab.buckets[0]
	}
	return tab.buckets[d-bucketMinDistance-1]
}
func (tab *Table) AddBootnode(n *enode.Node) {
	tab.setFallbackNodes([]*enode.Node{n})
}
// addSeenNode adds a node which may or may not be live to the end of a bucket. If the
// bucket has space available, adding the node succeeds immediately. Otherwise, the node is
// added to the replacements list.
//
// The caller must not hold tab.mutex.
func (tab *Table) addSeenNode(n *node) {
    //log.Info("42")
	//defer func(){log.Info("42.1")}()
	if n.ID() == tab.self().ID() {
		return
	}


	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	tab.allNodes[n.ID()] = n
	b := tab.bucket(n.ID())

	if b.entries.Contains( n.ID()) {
		// Already in bucket, don't add.
		return
	}

	if b.connects.Contains(n.ID()) {
		return
	}

	if b.replacements.Contains(n.ID()){
		return
	}
	if !tab.addIP(b, n.IP()) {
		// Can't add: IP limit reached.
		return
	}
	// Add to end of bucket:
	b.replacements.AddNode( n,false)

	n.addedAt = time.Now()
	if tab.nodeAddedHook != nil {
		tab.nodeAddedHook(n)
	}
}


// addVerifiedNode adds a node whose existence has been verified recently to the front of a
// bucket. If the node is already in the bucket, it is moved to the front. If the bucket
// has no space, the node is added to the replacements list.
//
// There is an additional safety measure: if the table is still initializing the node
// is not added. This prevents an attack where the table could be filled by just sending
// ping repeatedly.
//
// The caller must not hold tab.mutex.
func (tab *Table) addVerifiedNode(nodeId enode.ID) {
    //log.Info("43")
	//defer func(){log.Info("43.1")}()

	if !tab.isInitDone() {
		return
	}
	n,_ := tab.allNodes[nodeId]
	if n== nil {
		return
	}
	if n.ID() == tab.self().ID() {
		return
	}
	n.addedAt = time.Now()
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	b := tab.bucket(n.ID())

	if b.connects.Contains(n.ID()) {
		return
	}
	if  b.entries.Contains( n.ID()) {
		// Already in bucket, moved to front.
		b.entries.MoveFront(n.ID())
		return
	}
	if b.entries.AddNode(n,true) == false { //add不成功是因为队列满了，加入到replace中去
		if b.replacements.Length() >= bucketSize {
			b.replacements.ReplaceNode( n,func (oldNode *node) bool{
				if oldNode.latency > int64(10*time.Hour) {
					tab.removeIP(b, oldNode.IP())
					return true
				}
				return false
			})
			return
		}else{
			if !tab.addIP(b, n.IP()) {
				// Can't add: IP limit reached.
				return
			}
			// Bucket full, maybe add as replacement.
			b.replacements.AddNode( n,false)
			b.replacements.MoveFront(n.ID())
		}
	}else{
		//只有添加到了entries,才需要通知上层
		if tab.nodeAddedHook != nil {
			tab.nodeAddedHook(n)
		}
	}



}
/*
func (tab *Table) AddConnectedNode(oneNode *enode.Node) {
    //log.Info("44")
	//defer func(){log.Info("44.1")}()
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	if !tab.isInitDone() {
		return
	}

	if oneNode.ID() == tab.self().ID() {
		return
	}


	b := tab.bucket(oneNode.ID())

	if b.connects.Contains(oneNode.ID()) { //已经加入了，直接退出
		return
	}


	_,node1 := b.entries.RemoveNode(oneNode.ID())
	_,node2 := b.replacements.RemoveNode(oneNode.ID())

	n := tab.allNodes[oneNode.ID()]
	if n == nil {

		n = wrapNode(oneNode)
		log.Info("node warpped:","id",n.ID(),"ip",n.IP(),"udp",n.UDP())
	}
	log.Info("node from exist:","id",n.ID(),"ip",n.IP(),"udp",n.UDP())
	n.addedAt = time.Now()
	if node1 != nil {
		n = node1
	}
	if node2 != nil {
		n = node2
	}
	b.connects.AddNode(n,false)

}*/
func (tab *Table) TargetBucketInfo(nodeId enode.ID) (connects,entries,replacements *NodeQueue){
	bucket := tab.bucket(nodeId)
	return bucket.connects,bucket.entries,bucket.replacements
}

func (tab *Table) RemoveConnectedNode(oneNode *enode.Node) {
    log.Info("45")
	//defer func(){log.Info("45.1")}()
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	if !tab.isInitDone() {
		return
	}

	if oneNode.ID() == tab.self().ID() {
		return
	}

	b := tab.bucket(oneNode.ID())
	//前面anode在rlpx的时候，已经进行了一次不对称加密，所以是无法模仿出其他节点进来，因此判定一次IP只是冗余判定

	_,n := b.connects.RemoveNode(oneNode.ID())
	if n != nil {
		//从网络中断开的，移动到最后
		b.replacements.AddNode(n,true)
    //log.Info("node deleted :","id",	n.ID(),"addr",oneNode.IP(),"TCP",n.TCP(),"UDP",n.UDP())
	}else {
    //log.Info("unexpected node :","id",oneNode.ID(),"addr",oneNode.IP(),"TCP",oneNode.TCP(),"UDP",oneNode.UDP())
	}



}


func (tab *Table) addIP(b *bucket, ip net.IP) bool {
    //log.Info("46")
	//defer func(){log.Info("46.1")}()
	if netutil.IsLAN(ip) {
		return true
	}
	if !tab.ips.Add(ip) {
		log.Debug("IP exceeds table limit", "ip", ip)
		return false
	}
	if !b.ips.Add(ip) {
		log.Debug("IP exceeds bucket limit", "ip", ip)
		tab.ips.Remove(ip)
		return false
	}
	return true
}

func (tab *Table) removeIP(b *bucket, ip net.IP) {
    //log.Info("47")
	//defer func(){log.Info("47.1")}()
	if netutil.IsLAN(ip) {
		return
	}
	tab.ips.Remove(ip)
	b.ips.Remove(ip)
}



// nodesByDistance is a list of nodes, ordered by
// distance to target.
type nodesByDistance struct {
	entries []*node
	target  enode.ID
}
func (h *nodesByDistance)contains(nodeId enode.ID) bool {
	for _,v := range h.entries {
		if v.ID() == nodeId {
			return true
		}
	}
	return false
}
// push adds the given node to the list, keeping the total size below maxElems.
func (h *nodesByDistance) push(n *node, maxElems int) {
	ix := sort.Search(len(h.entries), func(i int) bool {
		return enode.DistCmp(h.target, h.entries[i].ID(), n.ID()) > 0
	})
	if len(h.entries) < maxElems {
		h.entries = append(h.entries, n)
	}
	if ix == len(h.entries) {
		// farther away than all nodes we already have.
		// if there was room for it, the node is now the last element.
	} else {
		// slide existing entries down to make room
		// this will overwrite the entry we just appended.
		copy(h.entries[ix+1:], h.entries[ix:])
		h.entries[ix] = n
	}
}
