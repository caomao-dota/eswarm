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
	"errors"
	"fmt"
	"github.com/plotozhu/MDCMainnet/p2p/enr"
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
	refreshInterval     = 10 * time.Minute //30 * time.Minute
	revalidateInterval  = 10 * time.Second
	copyNodesInterval   = 30 * time.Second
	seedMinTableTime    = 5 * time.Minute
	seedCount           = 30
	seedMaxAge          = 5 * 24 * time.Hour
)
type NodeItems struct {
	Id enode.ID
	Items map[encPubkey]*node
	mutex sync.Mutex
}
/*func newNodeItem(nodeId enode.ID) *NodeItems {
	return &NodeItems{
		Id:nodeId,Items:make(map[encPubkey]*node),
	}
}*/
func newNodeItem(n *node) *NodeItems {
	item := make(map[encPubkey]*node)
	item[encodePubkey(n.Pubkey())] = n
	return &NodeItems{
		Id:n.ID(),
		Items:item,
	}
}
func  (ni *NodeItems) ID() enode.ID {
	return ni.Id
}
func  (ni *NodeItems) AddNode(n *node,replace bool ) error {
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	return ni.doAddNode(n,replace)
}
func  (ni *NodeItems) doAddNode(n *node,replace bool ) error {

	if ni.Id != n.ID() {
		return errors.New(fmt.Sprintf("unmatched node ID: %v != %v ",ni.Id,n.ID()))
	}
	pubKey := encodePubkey(n.Pubkey())
	if _,ok := ni.Items[pubKey]; ok && !replace {
		return errors.New(fmt.Sprintf("NodeID %v with %v public key  exists",ni.Id,pubKey))
	}else{
		ni.Items[pubKey] = n
	}
	return nil
}
func  (ni *NodeItems)GetLastTestTime() time.Time{
	ni.mutex.Lock()
	defer ni.mutex.Unlock()

	if len(ni.Items) > 0 {

		for _, node := range ni.Items {
			return node.testAt
		}
	}
	return TimeInvalid
}
/**
	doPing对nodeItems里的每一项执行一次ping操作
 */
func  (ni *NodeItems)DoPing(t transport,ch chan bool){
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	go func (){
		if len(ni.Items) > 0 {

			for _,node := range ni.Items {
				node.testAt = time.Now()
				var toAddr net.UDPAddr
				lip := node.LIP()
				if len(lip) == 0 || node.LUDP() == 0{
					toAddr = net.UDPAddr{IP: node.IP(), Port: node.UDP()}
				}else {
					toAddr = net.UDPAddr{IP: lip, Port: int(node.LUDP())}
				}
				err,duration := t.ping(node.ID(),&toAddr)
				ni.mutex.Lock()
				if err == nil {
					ni.onPongSuccess(node,int64(duration))
				}else {
					ni.onPongFailed(node)
				}
				ni.mutex.Unlock()
			}
		}
		//pong 完了四种可能性,是否selected,是否OK,但是实际上，我们只关心selected的节点的状态
		/**
		 | selected | ok |
		 |  Y       |  N
		 |  N       | N
		 |  Y       |Y
		 |  Y       |N
		 */

		ni.mutex.Lock()
		 latency := LatencyInvalid
		for _,node := range ni.Items {
			if node.selected  {

				if node.latency == LatencyInvalid {
					node.selected = false
				}else {
					latency = node.latency
				}
				break
			} else {
				if node.latency < latency {
					latency = node.latency
				}
			}
		}

		ni.mutex.Unlock()
		ch <- latency != LatencyInvalid
	}()

}

func (ni *NodeItems)NodeExist(pubKey encPubkey) bool {
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	return ni.Items[pubKey] != nil
}
/**
	收到pong时候的处理，这个是用于在很久没有处理的节点，突然收到了ping，然后在udp中发送了一个ping以后的回应
    参数 n 节点
    duration,ping/pong 的延时
    ok 是否收到了pong
    ch 回调，如果某个被selected的节点，ping/pong还是成功的，那么返回true，如果某个被selected的节点失败了，返回false
 */
func (ni *NodeItems)DoPongResult(n *enode.Node,duration int64 , ok bool, ch chan bool )  {
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	pubkey := encodePubkey(n.Pubkey())
	node := ni.Items[pubkey]
	if node == nil {
		log.Error("Pong received but no ping recorded","id",n.ID(),"node",pubkey)
	}else {
		if ok {
			ni.onPongSuccess(node,int64(duration))
		}else {
			ni.onPongFailed(node)
		}
	}

	targetOK := false

	for _,node := range ni.Items {
		if node.selected  {
			targetOK =  node.latency != LatencyInvalid
			break
		}
	}


	ch <- targetOK

}
//在收到ping的时候，更新信息
func (ni *NodeItems)OnPingReceived(n *enode.Node,ip net.IP,port uint16) error{
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	if ni.Id != n.ID() {
		return errors.New(fmt.Sprintf("unmatched node ID: %v != %v ",ni.Id,n.ID()))
	}
	pubKey := encodePubkey(n.Pubkey())
	node := ni.Items[pubKey]
	newN := wrapNode(n)
	if node != nil {
		newN.latency = node.latency
		newN.addedAt = node.addedAt
		newN.findAt = time.Now()
		newN.testAt = node.testAt

	}else {
		newN.latency = LatencyInvalid
		newN.addedAt = time.Now()
		newN.findAt = time.Now()
		newN.testAt = TimeInvalid
	}
	newN.Node.Set(enr.LocalIP(ip))
	newN.Node.Set(enr.LUDP(port))
	ni.Items[pubKey] = newN
	return nil
}
//发送ping后收到了pong回应,证明这个网络是通的
//如果这个节点ping过存在，那么就返回nil，否则返回错误
func (ni *NodeItems)onPongSuccess(n *node,latency int64) error{
	if ni.Id != n.ID() {
		return errors.New(fmt.Sprintf("unmatched node ID: %v != %v ",ni.Id,n.ID()))
	}
	pubKey := encodePubkey(n.Pubkey())
	node := ni.Items[pubKey]

	if node != nil {
		node.latency = latency
	}else {
		return errors.New(fmt.Sprintf("pong receive without ping sent ID:%v, IP: %v,Port: %v ",ni.Id,n.LIP(),n.UDP()))
	}

	return nil
}

//发送ping后收到了pong回应,证明这个网络是通的
//如果这个节点ping过存在，那么就返回nil，否则返回错误
func (ni *NodeItems)onPongFailed(n *node) error{
	if ni.Id != n.ID() {
		return errors.New(fmt.Sprintf("unmatched node ID: %v != %v ",ni.Id,n.ID()))
	}
	pubKey := encodePubkey(n.Pubkey())
	node := ni.Items[pubKey]

	if node != nil {
		node.latency = LatencyInvalid
		if node.selected {
			node.selected = false
		}
	}else {
		return errors.New(fmt.Sprintf("pong receive without ping sent ID:%v, IP: %v,Port: %v ",ni.Id,n.LIP(),n.UDP()))
	}

	return nil
}
func  (ni *NodeItems) RemoveNode(pubKey encPubkey) error {
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	if _,ok := ni.Items[pubKey]; ok {
		delete(ni.Items,pubKey)
	}else{
		return errors.New(fmt.Sprintf("NodeID %v with %v public key  does not exist",ni.Id,pubKey))
	}
	return nil
}
func  (ni *NodeItems) SelectBest()*node {
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	node := ni.getPossibleNode()
	if node != nil {
		node.selected = true
	}else {
		log.Error("Error NodeItems","ID",ni.ID(),"items",len(ni.Items))
	}
	return node
}
/**
	获取最有可能的节点，要么是已经被选择的，要么是latency最短的，如果latency最短的都是LatencyInvalid，那么就返回一个nil
 */
func  (ni *NodeItems) GetPossibleNode()*node {
	ni.mutex.Lock()
	defer ni.mutex.Unlock()
	return ni.getPossibleNode()
}

func  (ni *NodeItems) getPossibleNode()*node {

	if len(ni.Items) == 0 {
		return nil
	}

	var result *node
	for _,node := range ni.Items {
		if node.selected {
			return node
		}
		if result == nil {
			result = node
		}else {
			if node.latency < result.latency {
				result = node
			}
		}

	}

	return result
}

type Table struct {
	mutex   sync.Mutex        // protects buckets, bucket content, nursery, rand
	buckets [nBuckets]*bucket // index of known nodes by distance
	nursery []*node           // bootstrap nodes
	rand    *mrand.Rand       // source of randomness, periodically reseeded
	ips     netutil.DistinctNetSet

	db         *enode.DB // database of known nodes
	net        transport
	refreshReq chan chan struct{}
	initDone   bool
	waiting   chan struct{}
	closeOnce sync.Once
	closeReq  chan struct{}
	closed    chan struct{}
	notifyChannel    chan struct{}
	nodeAddedHook func(*node) // for testing
	//allNodes     map[enode.ID]*NodeItems

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
	entries  []*NodeItems
	exists   map[enode.ID]*NodeItems
	maxsize int
	mutex   sync.Mutex
}

func NewNodeQueue(maxsize int) *NodeQueue{
	return &NodeQueue{
		entries:make([]*NodeItems,0),
		exists:make(map[enode.ID]*NodeItems),
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

	nq.entries=make([]*NodeItems,0)
	nq.exists=make(map[enode.ID]*NodeItems)
	nq.mutex.Unlock()
	for _,anode := range nodes {
		nq.AddNode(anode,false)
	}
}

func (nq *NodeQueue)getNodeByIndex(index int) *node{
	nq.mutex.Lock()

	nq.mutex.Unlock()
	if len(nq.entries) > index {
		return nq.entries[index].GetPossibleNode()
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

func (nq *NodeQueue)ReplaceNodeItems(anode *NodeItems, checkReplace func(nodeInEntries *NodeItems) bool) (bool,*NodeItems) {
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
    //log.Info("1")
	//defer func(){log.Info("1.1")}()
	for n,oldNode := range nq.entries {

		if checkReplace(oldNode)  {
			nq.entries[n] = anode
			delete(nq.exists,oldNode.ID())
			nq.exists[anode.ID()] = anode

			return  true,oldNode
		}
	}
	return false,nil

}
func (nq *NodeQueue)AddNodeItems(anode *NodeItems,shouldUpdate bool) bool {
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
	return nq.doAddNodeItems(anode,shouldUpdate)
}
func (nq *NodeQueue)doAddNodeItems(anode *NodeItems,shouldUpdate bool) bool{

	//log.Info("3")
	//defer func(){log.Info("3.1")}()
	_,ok := nq.exists[anode.ID()]
	if ok {
		if shouldUpdate {
			for n,node := range nq.entries {
				if node.ID() == anode.ID() {
					nq.entries[n] = anode
					nq.exists[anode.ID()] = anode

					return true
				}
			}
			log.Error("Error in nq.entries","nodeId",anode.ID(),"reason","node exist in exists,but not in entries")
			delete(nq.exists,anode.ID())
			return false //update failed, should not occur
		}
		return true
	}else {
		if len(nq.entries) >= nq.maxsize {
			return false
		}else {
			nq.entries = append(nq.entries,anode)
			nq.exists[anode.ID()] = anode
			return  true
		}
	}
}
func (nq *NodeQueue)AddNode(anode *node,shouldUpdate bool) (bool, *NodeItems){
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
    //log.Info("3")
	//defer func(){log.Info("3.1")}()
	nodeItems,ok := nq.exists[anode.ID()]
	if ok {
		nodeItems.AddNode(anode,shouldUpdate)
		return true,nodeItems
	}else {
		if  len (nq.entries) < nq.maxsize {
			nodeItems = newNodeItem(anode)

			nq.doAddNodeItems(nodeItems,shouldUpdate)
			return true,nodeItems
		}
		return false,nil
	}
}
func (nq *NodeQueue)GetEntries() []*NodeItems{
   // log.Info("4")
	//defer func(){log.Info("4.1")}()
	nq.mutex.Lock()
	defer nq.mutex.Unlock()
	result := make([]*NodeItems,len(nq.entries))
	for i := 0; i < len(nq.entries); i++ {
		result[i] = nq.entries[i]
	}
	return result
}
func (nq *NodeQueue)RemoveNodeItems(nodeId enode.ID) (bool,*NodeItems){
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
func (nq *NodeQueue)Get(nodeId enode.ID) *NodeItems {
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
			nq.entries = append([]*NodeItems{anode},nq.entries...)
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

	entries     *NodeQueue // live entries, sorted by time of last contact
	replacements *NodeQueue // recently seen nodes to be used if revalidation fails
	ips          netutil.DistinctNetSet
}

func newTable(t transport, db *enode.DB, bootnodes []*enode.Node) (*Table, error) {
	tab := &Table{
		net:        t,
		db:         db,
		refreshReq: make(chan chan struct{}),
		initDone:   false,
		waiting:    make(chan struct{}),
		closeReq:   make(chan struct{}),
		closed:     make(chan struct{}),
		rand:       mrand.New(mrand.NewSource(0)),
		ips:        netutil.DistinctNetSet{Subnet: tableSubnet, Limit: tableIPLimit},
		//allNodes:   make(map[enode.ID]*NodeItems),
	}
	if err := tab.setFallbackNodes(bootnodes); err != nil {
		return nil, err
	}
	for i := range tab.buckets {
		tab.buckets[i] = &bucket{
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
	log.Info("lock7")

	tab.rand.Seed(int64(binary.BigEndian.Uint64(b[:])))
	tab.mutex.Unlock()
	log.Info("unlock7")
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
	log.Info("lock8")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock8")}()

	// Find all non-empty buckets and get a fresh slice of their entries.
	var buckets [][]*NodeItems
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
		buf[i] = unwrapNode(b[0].SelectBest())
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
	if !tab.initDone {
		if tab.waiting  != nil {
			select {
			case <- tab.waiting:
				return true
			default:
				return false
			}
		}

	}
	return true
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
	log.Info("lock9")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock9")}()
    //log.Info("13")
	//defer func(){log.Info("13.1")}()
	ret := make(SortableNode,0)
	//log.Debug("step 1")
	for _,bucket := range tab.buckets {
		bucketRet := make(SortableNode,0)
		for _,nodeItems := range bucket.entries.GetEntries() {
			node := nodeItems.SelectBest()
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
	log.Info("lock10")

	cl := tab.closest(hash, 1)
	tab.mutex.Unlock()
	log.Info("unlock10")
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
		log.Info("lock 11")
		// generate initial result set
		result = tab.closest(target, bucketSize)
		tab.mutex.Unlock()
		log.Info("unlock 11")
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
	udpTarget := n.addr()
	if !n.LIP().Equal(net.IP{}) {
		udpTarget = &net.UDPAddr{IP:n.LIP(),Port:int(n.LUDP())}
	}
	r, err := tab.net.findnode(n.ID(),udpTarget , targetKey)
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
	log.Info("lock12")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock12")}()
    //log.Info("19")
	//defer func(){log.Info("19.1")}()
	knownCount := 0
	for _,bucket := range tab.buckets {

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

	)
	defer refresh.Stop()
	defer revalidate.Stop()
	defer copyNodes.Stop()
	defer replace.Stop()
	// Start initial refresh.
	go func (){
		tab.doRefresh(refresh)

		go tab.doReplacementCheck()

	}()


loop:
	for {
		select {
		case <-refresh.C:
			tab.seedRand()
			go tab.doRefresh(refresh)
		case  <-tab.refreshReq:
			go tab.doRefresh(refresh)
		case <-revalidate.C:
			go tab.doRevalidate(revalidate)
		case <-replace.C:
			go tab.doReplacementCheck()
		case <-copyNodes.C:
			go tab.copyLiveNodes()
		case <-tab.closeReq:
			break loop
		}
	}


    //log.Info("Do tab finished")
	close(tab.closed)
}


// doRefresh performs a lookup for a random target to keep buckets
// full. seed nodes are inserted if the table is empty (initial
// bootstrap or discarded faulty peers).
func (tab *Table) doRefresh(refresh *time.Timer) {
    //log.Info("31")
	//defer func(){log.Info("31.1")}()
	defer func (){
		if !tab.initDone  {
			tab.initDone = true
			if tab.waiting != nil  {
				close(tab.waiting)
				tab.waiting = nil
			}
		}
		refresh.Reset(tab.setupRefreshTime())
	}()


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
func (tab *Table) RegisterPeer(node2 enode.Node) {
	//发送了ping包，如果收到了pong包，就会在数据库里有记录
	tab.net.ping(node2.ID(),&net.UDPAddr{IP: node2.IP(), Port: node2.UDP()})
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
		seed.Set(enr.LocalIP(nil))
		seed.latency =tab.db.GetNodeLatency(seed.ID(),seed.IP())
		age := log.Lazy{Fn: func() interface{} { return time.Since(tab.db.LastPongReceived(seed.ID(), seed.IP())) }}
		log.Trace("Found seed node in database", "id", seed.ID(), "addr", seed.addr(), "age", age,"latency",seed.latency)
		tab.addSeenNode(seed)
	}
}
const (
	StateUnknown = 0
	StateEntries =1
	StateReplace = 2
)
func (tab *Table)updateNodeStatus(nodeId enode.ID,b *bucket,alive bool ){
    //log.Info("33")
	//defer func(){log.Info("33.1")}()
	//b := tab.buckets[bi]

	var anode  *NodeItems
	anode = nil
	//state 0 not found /1 in  connects /2 in entries /3 in replacement

	if anode = b.entries.Get(nodeId); anode != nil {
		if ! alive {
			//移动到replacement的最后
			_,items := b.entries.RemoveNodeItems(nodeId)
			if b.replacements.Length() >0 && time.Since( b.replacements.GetEntries()[0].GetLastTestTime()) > RevalidateInterval{
				ok,items := b.replacements.RemoveNodeItems(b.replacements.GetEntries()[0].ID())
				if ok {
					b.entries.AddNodeItems(items,false)
				}

			}
			b.replacements.AddNodeItems(items,true)
		}else{
			b.entries.MoveFront(nodeId)
		}

	} else if anode = b.replacements.Get(nodeId); anode != nil {
		if alive {
			_,items := b.replacements.RemoveNodeItems(nodeId)
			b.entries.AddNodeItems(items,true)
			b.entries.MoveFront(items.ID())
			if tab.nodeAddedHook != nil {
				tab.nodeAddedHook(items.SelectBest())
			}

		}else {
			b.replacements.MoveBack(nodeId)
		}
	} else {
		//新的东西，其实不应该出现
		log.Error("Received an unknown node respone")
	}


}
func (tab *Table)UpdateIPInfo(nodeId enode.ID,oldIP net.IP,newIP net.IP){
	tab.mutex.Lock()
	log.Info("lock13")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock13")}()
	b := tab.bucket(nodeId)
	node := b.entries.Get(nodeId)
	if node == nil  {
		node = b.replacements.Get(nodeId)
	}

	if node != nil {
		for _,n := range node.Items {
			if n.IP().Equal(oldIP) {
				n.Node.Record().Set(enr.IP(newIP))
				break;
			}
		}
	}
}

// doRevalidate checks that the last node in a random bucket is still live
// and replaces or deletes the node if it isn't.
func (tab *Table) doReplacementCheck() {
//	defer func() { done <- struct{}{} }()
    //log.Info("34")
	//defer func(){log.Info("34.1")}()
	last, bi := tab.replaceNodeToCheck()
	if last == nil {
		// No non-empty bucket found.
		return
	}

	// Ping the selected node and wait for a pong.
	ch := make(chan bool )
	last.DoPing(tab.net,ch)
	//如果测试成功了，移动到etnries的最前面

	result := <-ch
	tab.updateNodeStatus(last.ID(),tab.buckets[bi],result)
}

// doRevalidate checks that the last node in a random bucket is still live
// and replaces or deletes the node if it isn't.
func (tab *Table) doRevalidate( revalidate *time.Timer) {
	defer func() { revalidate.Reset(tab.nextRevalidateTime())}()

    //log.Info("35")
	//defer func(){log.Info("35.1")}()
	//all connected is set to seen now
	tab.mutex.Lock()
	log.Info("lock 14")

	last, bi := tab.nodeToRevalidate()
	tab.mutex.Unlock()
	log.Info("unlock 14")
	if last == nil {
		// No non-empty bucket found.
		return
	}

	// Ping the selected node and wait for a pong. it will use processLive来进行结果处理
	//tab.net.ping(last.ID(), last.addr())]
	ch := make(chan bool )
	last.DoPing(tab.net,ch)
	//如果测试失败了，移动到replacement的最后面
	result := <-ch
	tab.updateNodeStatus(last.ID(),tab.buckets[bi],result)
}

// nodeToRevalidate returns the last node in a random, non-empty bucket.
func (tab *Table) nodeToRevalidate() (n *NodeItems, bi int) {

    //log.Info("36")
	//defer func(){log.Info("36.1")}()
	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if b.entries.Length() > 0 {
			last := b.entries.GetEntries()[b.entries.Length()-1]
			if time.Since(last.GetLastTestTime()) > 30 *time.Second {
				return last,bi
			}else{
				return nil,0
			}
		}
	}
	return nil, 0
}
// nodeToRevalidate returns the last node in a random, non-empty bucket.
func (tab *Table) replaceNodeToCheck() (n *NodeItems, bi int) {

    //log.Info("37")
	//defer func(){log.Info("37.1")}()
	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if b.entries.Length() < bucketSize && b.replacements.Length() > 0 {
			last := b.replacements.GetEntries()[0]
			if time.Since(last.GetLastTestTime()) > 30 *time.Second {
				return last, bi
			}else{
				return nil,0
			}

		}
	}
	return nil, 0
}
func (tab *Table) nextRevalidateTime() time.Duration {

    //log.Info("38")
	//defer func(){log.Info("38.1")}()
	return time.Duration(tab.rand.Int63n(int64(revalidateInterval)))
}

// copyLiveNodes adds nodes from the table to the database if they have been in the table
// longer then minTableTime.
func (tab *Table) copyLiveNodes() {

    //log.Info("39")
	//defer func(){log.Info("39.1")}()
	now := time.Now()
	for _, b := range &tab.buckets {
		for _, n := range b.entries.GetEntries() {
			for _,node := range n.Items {
				value := now.Sub(node.addedAt)
				if  value >= seedMinTableTime {
					tab.db.UpdateNode(unwrapNode(node))
				}
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
    //log.Info("40")
	//defer func(){log.Info("40.1")}()
	close := &nodesByDistance{target: target}
	for _, b := range &tab.buckets {
		for _, n := range b.entries.GetEntries() {
			node := n.SelectBest()
			if node != nil {
				if (node.testAt) != TimeInvalid{
					close.push(n.SelectBest(), nresults)
				}
			}


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

func (tab *Table) AddSeenNode(n *node) *NodeItems{
	tab.mutex.Lock()
	log.Info("lock1")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock1")}()
	return tab.addSeenNode(n)
}
func (tab *Table) addSeenNode(n *node) *NodeItems{
    //log.Info("42")
	//defer func(){log.Info("42.1")}()
	if n.ID() == tab.self().ID() {
		return nil
	}




	/*nodes,ok := tab.allNodes[n.ID()]
	if !ok  {
		nodes = newNodeItem(n.ID())
		tab.allNodes[n.ID()] = nodes
	}

	nodes.AddNode(n,false)
	*/
	b := tab.bucket(n.ID())

	//fmt.Println(fmt.Sprintf("add seen node,curent:%v,max:%v",b.entries.Length(),b.entries.maxsize))
	if b.entries.Contains( n.ID()) {
		// Already in bucket, don't add.
		result := b.entries.Get(n.ID())
		 result.AddNode(n,false)
		return  result

	}


	if b.replacements.Contains(n.ID()){
		result :=b.replacements.Get(n.ID())
		 result.AddNode(n,false)
		return result

	}
	if !tab.addIP(b, n.IP()) {
		// Can't add: IP limit reached.
		return nil
	}

	var result *NodeItems
	if b.entries.Length() < b.entries.maxsize {
		_,result = b.entries.AddNode(n,false)
	}else {
		if n.testAt != TimeInvalid {
			nodeItem := newNodeItem(n)

			ok,replaced := b.entries.ReplaceNodeItems(nodeItem, func(nodeInEntries *NodeItems) bool {
				if nodeInEntries.GetLastTestTime() == TimeInvalid {
					return true
				}
				return false
			})
			if ok {
				b.replacements.AddNodeItems( replaced,false)
				result = replaced
			}
		}else {

			// Add to end of bucket:
			_,result = b.replacements.AddNode( n,false)
		}

	}


	n.addedAt = time.Now()
	return result

}

func (tab *Table) DoPing(n  *enode.Node) {


	toTest := tab.AddSeenNode(wrapNode(n))
	ch := make(chan bool)
	toTest.DoPing(tab.net,ch)
	result := <- ch
	tab.updateNodeStatus(toTest.ID(),tab.bucket(toTest.ID()),result)
}
func (tab *Table) OnPingReceived(n  *enode.Node,ip net.IP,port uint16) {
    //log.Info("43")
	//defer func(){log.Info("43.1")}()

	if n.ID() == tab.self().ID() {
		return
	}

	tab.mutex.Lock()
	log.Info("lock2")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock2")}()

	b := tab.bucket(n.ID())

	if  b.entries.Contains( n.ID()) {

		oldNode := b.entries.Get(n.ID())
		oldNode.OnPingReceived(n,ip,port)
		return

	} else if b.replacements.Contains(n.ID()) {
		oldNode := b.replacements.Get(n.ID())
		oldNode.OnPingReceived(n,ip,port)
		ch := make(chan bool )
		oldNode.DoPing(tab.net,ch)
		//如果测试失败了，移动到replacement的最后面
		result := <-ch
		tab.updateNodeStatus(oldNode.ID(),tab.bucket(oldNode.ID()),result)


	} else {

		_,oldNode := b.replacements.AddNode(wrapNode(n),true)
		if oldNode != nil {
			ch := make(chan bool )
			oldNode.DoPing(tab.net,ch)
			//如果测试失败了，移动到replacement的最后面
			result := <-ch
			tab.updateNodeStatus(oldNode.ID(),tab.bucket(oldNode.ID()),result)
		}

	}



}
func (tab *Table) DoPongResult(n *enode.Node,duration int64,ok bool)  {


	tab.mutex.Lock()
	log.Info("lock3")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock3")}()

	if n.ID() == tab.self().ID() {
		return
	}
	b := tab.bucket(n.ID())


	ch := make(chan bool )
	if  b.entries.Contains( n.ID()) {

		oldNode := b.entries.Get(n.ID())
		oldNode.DoPongResult(n,duration,ok,ch)

	} else if b.replacements.Contains(n.ID()) {
		oldNode := b.replacements.Get(n.ID())
		oldNode.DoPongResult(n,duration,ok,ch)

	} else {
		b.replacements.AddNode(wrapNode(n),true)
		oldNode := b.replacements.Get(n.ID())
		oldNode.DoPongResult(n,duration,ok,ch)
	}
	result := <- ch

	tab.updateNodeStatus(n.ID(),b,result)

}
/**
    当收到其他节点的连接时，通过这个来检查是否允许连接，而实际上，在连接的之前，都应该使用ping/pong测试过才可以
    如果我没有收到过ping/pong的测试，那么就禁止连接
 */
func (tab *Table) CanAddNode(n *enode.Node) (bool) {
    //log.Info("44")
	//defer func(){log.Info("44.1")}()


	b := tab.bucket(n.ID())

	//tab.OnPingReceived(n)
	tab.mutex.Lock()
	log.Info("lock4")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock4")}()
	if  b.entries.Contains( n.ID()) {
		nodes := b.entries.Get(n.ID())
		return nodes.NodeExist(encodePubkey(n.Pubkey()))
	}


	return false

}
func (tab *Table) TargetBucketInfo(nodeId enode.ID) (bucketId int,entries,replacements *NodeQueue){
	tab.mutex.Lock()
	log.Info("lock5")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock5")}()
	bucket := tab.bucket(nodeId)
	bucketIndex := 0
	//defer func(){log.Info("41.1")}()
	d := enode.LogDist(tab.self().ID(), nodeId)
	if d <= bucketMinDistance {
		bucketIndex = 0
	}else {
		bucketIndex = d-bucketMinDistance-1
	}

	return bucketIndex,bucket.entries,bucket.replacements
}

func (tab *Table) RemoveConnectedNode(nodeId enode.ID) {
    log.Info("Node Disconnected:","id",nodeId)
	//defer func(){log.Info("45.1")}()
	tab.mutex.Lock()
	log.Info("lock6")
	defer func (){ tab.mutex.Unlock(); 	log.Info("unlock6")}()


	if nodeId == tab.self().ID() {
		return
	}


	b := tab.bucket(nodeId)
	tab.updateNodeStatus(nodeId,b,false)

	//前面anode在rlpx的时候，已经进行了一次不对称加密，所以是无法模仿出其他节点进来，因此判定一次IP只是冗余判定


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
