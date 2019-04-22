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
}

// transport is implemented by the UDP transport.
// it is an interface so we can test without opening lots of UDP
// sockets and without generating a private key.
type transport interface {
	self() *enode.Node
	ping(enode.ID, *net.UDPAddr) error
	findnode(toid enode.ID, addr *net.UDPAddr, target encPubkey) ([]*node, error)
	close()
}

// bucket contains nodes, ordered by their last activity. the entry
// that was most recently active is the first element in entries.
type bucket struct {
	connects     []*node
	entries      []*node // live entries, sorted by time of last contact
	replacements []*node // recently seen nodes to be used if revalidation fails
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
	}
	if err := tab.setFallbackNodes(bootnodes); err != nil {
		return nil, err
	}
	for i := range tab.buckets {
		tab.buckets[i] = &bucket{
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
	if !tab.isInitDone() {
		return 0
	}
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	// Find all non-empty buckets and get a fresh slice of their entries.
	var buckets [][]*node
	for _, b := range &tab.buckets {
		if len(b.entries) > 0 {
			buckets = append(buckets, b.entries)
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
	ret := make(SortableNode,0)
	//log.Debug("step 1")
	for _,bucket := range tab.buckets {
		bucketRet := make(SortableNode,0)
		for _,node := range bucket.entries {
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
	//log.Debug("step 3")
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
	return unwrapNodes(tab.lookup(target, true))
}

func (tab *Table)ProcessLive(nodeId enode.ID, isLive bool) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	bucket :=tab.bucket(nodeId)
	tab.execValidateNode(nodeId,bucket,isLive)
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

			if !asked[n.ID()] && !enode.IsLightNode(enode.NodeTypeOption(n.NodeType()) ) {
				asked[n.ID()] = true
				pendingQueries++
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
		if fails >= maxFindnodeFailures {
			log.Trace("Too many findnode failures, dropping", "id", n.ID(), "failcount", fails)
			tab.delete(n)
		}
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
	knownCount := 0
	for _,bucket := range tab.buckets {
		knownCount += len(bucket.connects)
		knownCount += len(bucket.entries)
	}
	if knownCount < 5 {
		return 10 * time.Second
	}else if  knownCount <= 100  {
		return time.Duration(10 + 10*knownCount) * time.Second
	}else {
		return 30*time.Minute
	}
}
func (tab *Table) refresh() <-chan struct{} {
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
	go tab.doRefresh(refreshDone)

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
	close(tab.closed)
}

// doRefresh performs a lookup for a random target to keep buckets
// full. seed nodes are inserted if the table is empty (initial
// bootstrap or discarded faulty peers).
func (tab *Table) doRefresh(done chan struct{}) {
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
	seeds := wrapNodes(tab.db.QuerySeeds(seedCount, seedMaxAge))

	seeds = append(seeds, tab.nursery...)
	for i := range seeds {
		seed := seeds[i]
		seed.latency =tab.db.GetNodeLatency(seed.ID(),seed.IP())
		age := log.Lazy{Fn: func() interface{} { return time.Since(tab.db.LastPongReceived(seed.ID(), seed.IP())) }}
		log.Trace("Found seed node in database", "id", seed.ID(), "addr", seed.addr(), "age", age)
		tab.addSeenNode(seed)
	}
}
func (tab *Table)execValidateNode(nodeId enode.ID,b *bucket,alive bool ){

	//b := tab.buckets[bi]
	var anode  *node
	anode = nil
	//state 0 not found /1 in  connects /2 in entries /3 in replacement
	state := 0
	for _,node := range b.connects {
		if node.ID() == nodeId {
			anode = node
			state = 1
			break;
		}
	}
	if anode == nil { // 不在connected中
		for _,node := range b.entries {
			if node.ID() == nodeId {
				anode = node
				state = 2
				break;
			}
		}
		if anode == nil { //也不在enties里

			for _,node := range b.replacements {
				if node.ID() == nodeId {
					anode = node
					state = 3
					break;
				}
			}

		}
	}

	if alive {
		if state == 2 || state == 3 {
		 		//在entries中
		 		if state == 2 {
					//移动到entries的最前面
					tab.bumpInBucket(b,anode)
				}else {
					b.entries = append([]*node{anode},b.entries...)
				}

				//保障，从replacements中删除
				deleteNode(b.replacements,anode)

		}	else if  state == 0 {
				log.Error("Error node ping????","node ID",nodeId)


		} else {

		}
	} else {

		if state != 0 {
			b.connects = deleteNode(b.connects,anode) //从connect中移除（假如有）
			b.entries = deleteNode(b.entries,anode)//从entries中移除（假如有）
			//保障删除
			b.replacements = deleteNode(b.replacements,anode)//从replacement中移除（假如有）
			//断开的连接移动到最后
			b.replacements = append(b.replacements,anode)
		} else {
			log.Error("Error node ping????","node ID",nodeId)
		}

	}

}

// doRevalidate checks that the last node in a random bucket is still live
// and replaces or deletes the node if it isn't.
func (tab *Table) doReplacementCheck(done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	last, bi := tab.replaceNodeToCheck()
	if last == nil {
		// No non-empty bucket found.
		return
	}

	// Ping the selected node and wait for a pong.
	err := tab.net.ping(last.ID(), last.addr())

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	tab.execValidateNode(last.ID(),tab.buckets[bi],err == nil )
}

// doRevalidate checks that the last node in a random bucket is still live
// and replaces or deletes the node if it isn't.
func (tab *Table) doRevalidate(done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	last, bi := tab.nodeToRevalidate()
	if last == nil {
		// No non-empty bucket found.
		return
	}

	// Ping the selected node and wait for a pong.
	err := tab.net.ping(last.ID(), last.addr())

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	tab.execValidateNode(last.ID(),tab.buckets[bi],err == nil )
}

// nodeToRevalidate returns the last node in a random, non-empty bucket.
func (tab *Table) nodeToRevalidate() (n *node, bi int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if len(b.entries) > 0 {
			last := b.entries[len(b.entries)-1]
			return last, bi
		}
	}
	return nil, 0
}
// nodeToRevalidate returns the last node in a random, non-empty bucket.
func (tab *Table) replaceNodeToCheck() (n *node, bi int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if len(b.entries) + len(b.connects) < bucketSize && len(b.replacements) > 0 {
			last := b.replacements[0]
			return last, bi
		}
	}
	return nil, 0
}
func (tab *Table) nextRevalidateTime() time.Duration {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	return time.Duration(tab.rand.Int63n(int64(revalidateInterval)))
}

// copyLiveNodes adds nodes from the table to the database if they have been in the table
// longer then minTableTime.
func (tab *Table) copyLiveNodes() {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	now := time.Now()
	for _, b := range &tab.buckets {
		for _, n := range b.entries {
			if n.isLiving && now.Sub(n.addedAt) >= seedMinTableTime {
				tab.db.UpdateNode(unwrapNode(n))
			}
		}
	}
}

// closest returns the n nodes in the table that are closest to the
// given id. The caller must hold tab.mutex.
func (tab *Table) closest(target enode.ID, nresults int) *nodesByDistance {
	// This is a very wasteful way to find the closest nodes but
	// obviously correct. I believe that tree-based buckets would make
	// this easier to implement efficiently.
	close := &nodesByDistance{target: target}
	for _, b := range &tab.buckets {
		for _, n := range b.entries {
			if n.isLiving  {
				close.push(n, nresults)
			}
		}
	}
	return close
}

func (tab *Table) len() (n int) {
	for _, b := range &tab.buckets {
		n += len(b.entries)
	}
	return n
}

// bucket returns the bucket for the given node ID hash.
func (tab *Table) bucket(id enode.ID) *bucket {
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
	if n.ID() == tab.self().ID() {
		return
	}

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	b := tab.bucket(n.ID())
	if contains(b.entries, n.ID()) {
		// Already in bucket, don't add.
		return
	}

	if contains(b.connects,n.ID()) {
		return
	}

	if contains(b.replacements,n.ID()){
		return
	}
	if !tab.addIP(b, n.IP()) {
		// Can't add: IP limit reached.
		return
	}
	// Add to end of bucket:
	b.replacements = append(b.replacements, n)

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
func (tab *Table) addVerifiedNode(n *node) {
	n.isLiving = true
	if !tab.isInitDone() {
		return
	}
	if n.ID() == tab.self().ID() {
		return
	}

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	b := tab.bucket(n.ID())

	if contains(b.connects,n.ID()) {
		return
	}
	if tab.bumpInBucket(b, n) {
		// Already in bucket, moved to front.
		return
	}
	if len(b.entries) >= bucketSize {
		// Bucket full, maybe add as replacement.
		tab.addReplacement(b, n)
		return
	}
	if !tab.addIP(b, n.IP()) {
		// Can't add: IP limit reached.
		return
	}
	// Add to front of bucket.
	b.entries, _ = pushNode(b.entries, n, bucketSize)
	b.replacements = deleteNode(b.replacements, n)
	n.addedAt = time.Now()
	if tab.nodeAddedHook != nil {
		tab.nodeAddedHook(n)
	}
}
func (tab *Table) AddConnectedNode(node *enode.Node) {

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	if !tab.isInitDone() {
		return
	}
	n := wrapNode(node)
	if n.ID() == tab.self().ID() {
		return
	}


	b := tab.bucket(n.ID())

	if contains(b.connects,n.ID()) { //已经加入了，直接退出
		return
	}
	if contains(b.entries,n.ID()){ //在备选队列中，从备选队列中删除
		//remove from entries
		b.entries = deleteNode(b.entries,n)
	}

	if contains(b.replacements,n.ID()) {  //在后备队列中，从后备队列中删除
		b.replacements = deleteNode(b.replacements, n)
	}

	// Add to front of bucket.
	b.connects, _ = pushNode(b.connects, n, len(b.connects)+1) //保证连接成功
	n.addedAt = time.Now()
	if tab.nodeAddedHook != nil {
		tab.nodeAddedHook(n)
	}
}
func (tab *Table) RemoveConnectedNode(node *enode.Node) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	if !tab.isInitDone() {
		return
	}
	n := wrapNode(node)
	if n.ID() == tab.self().ID() {
		return
	}


	b := tab.bucket(n.ID())

	if contains(b.connects,n.ID()) { //已经加入了，直接退出
		b.connects = deleteNode(b.connects,n)
	} else if  contains(b.entries,n.ID()){ //在备选队列中，从备选队列中删除
		//remove from entries
		b.entries = deleteNode(b.entries,n)
	} else if contains(b.replacements,n.ID()){
		b.replacements = deleteNode(b.replacements,n)
	}

	//从网络中断开的，移动到最后
	b.replacements = append(b.replacements,n)


}
// delete removes an entry from the node table. It is used to evacuate dead nodes.
func (tab *Table) delete(node *node) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	tab.deleteInBucket(tab.bucket(node.ID()), node)
}

func (tab *Table) addIP(b *bucket, ip net.IP) bool {
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
	if netutil.IsLAN(ip) {
		return
	}
	tab.ips.Remove(ip)
	b.ips.Remove(ip)
}

func (tab *Table) addReplacement(b *bucket, n *node) {
	for _, e := range b.replacements {
		if e.ID() == n.ID() {
			return // already in list
		}
	}
	if !tab.addIP(b, n.IP()) {
		return
	}
	var removed *node
	b.replacements, removed = pushNode(b.replacements, n, maxReplacements)
	if removed != nil {
		tab.removeIP(b, removed.IP())
	}
}

// replace removes n from the replacement list and replaces 'last' with it if it is the
// last entry in the bucket. If 'last' isn't the last entry, it has either been replaced
// with someone else or became active.
func (tab *Table) replace(b *bucket, last *node) *node {
	for index,node := range b.entries {
		if node.ID() == last.ID(){
			// Still the last entry.
			if len(b.replacements) == 0 {
				tab.deleteInBucket(b, last)
				return nil
			}
			b.entries = append(b.entries[0:index],b.entries[index+1:]...)
			r := b.replacements[tab.rand.Intn(len(b.replacements))]
			r.isLiving = false //确保正确性，其实移动到replace的时候，已经做了
			b.replacements = deleteNode(b.replacements, r)
			b.entries = append(b.entries,r)
			tab.removeIP(b, last.IP())
			return r
		}
	}
	return nil

}

// bumpInBucket moves the given node to the front of the bucket entry list
// if it is contained in that list.
func (tab *Table) bumpInBucket(b *bucket, n *node) bool {
	for i := range b.entries {
		if b.entries[i].ID() == n.ID() {
			if !n.IP().Equal(b.entries[i].IP()) {
				// Endpoint has changed, ensure that the new IP fits into table limits.
				tab.removeIP(b, b.entries[i].IP())
				if !tab.addIP(b, n.IP()) {
					// It doesn't, put the previous one back.
					tab.addIP(b, b.entries[i].IP())
					return false
				}
			}
			// Move it to the front.
			copy(b.entries[1:], b.entries[:i])
			b.entries[0] = n
			return true
		}
	}
	return false
}

func (tab *Table) deleteInBucket(b *bucket, n *node) {
	b.entries = deleteNode(b.entries, n)
	tab.removeIP(b, n.IP())
}
/**
	某个bucket里是否有对应的ID
 */
func contains(ns []*node, id enode.ID) bool {
	for _, n := range ns {
		if n.ID() == id {
			return true
		}
	}
	return false
}

// pushNode adds n to the front of list, keeping at most max items.
func pushNode(list []*node, n *node, max int) ([]*node, *node) {
	if len(list) < max {
		list = append(list, nil)
	}
	removed := list[len(list)-1]
	copy(list[1:], list)
	list[0] = n
	return list, removed
}

// deleteNode removes n from list.
func deleteNode(list []*node, n *node) []*node {
	for i := range list {
		if list[i].ID() == n.ID() {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// nodesByDistance is a list of nodes, ordered by
// distance to target.
type nodesByDistance struct {
	entries []*node
	target  enode.ID
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
