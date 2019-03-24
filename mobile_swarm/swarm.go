// Copyright 2016 The go-ethereum Authors
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

// Contains all the wrappers from the node package to support client side node
// management on mobile platforms.

package swarm

//import "C"
import (
	"fmt"
	"github.com/plotozhu/MDCMainnet/accounts"
	"github.com/plotozhu/MDCMainnet/accounts/keystore"
	"github.com/plotozhu/MDCMainnet/common"
	"github.com/plotozhu/MDCMainnet/node"
	"github.com/plotozhu/MDCMainnet/p2p"
	"github.com/plotozhu/MDCMainnet/p2p/nat"
	"github.com/plotozhu/MDCMainnet/swarm"
	bzzapi "github.com/plotozhu/MDCMainnet/swarm/api"
	"io/ioutil"
	"path/filepath"
)

// NodeConfig represents the collection of configuration values to fine tune the Geth
// node embedded into a mobile process. The available values are a subset of the
// entire API provided by go-ethereum to reduce the maintenance surface and dev
// complexity.
type NodeConfig struct {
	// Bootstrap nodes used to establish connectivity with the rest of the network.
	//BootstrapNodes *Enodesv4

	// MaxPeers is the maximum number of peers that can be connected. If this is
	// set to zero, then only the configured static and trusted peers can connect.
	MaxPeers int

	// EthereumEnabled specifies whether the node should run the Ethereum protocol.
	EthereumEnabled bool

	// EthereumNetworkID is the network identifier used by the Ethereum protocol to
	// decide if remote peers should be accepted or not.
	EthereumNetworkID int64 // uint64 in truth, but Java can't handle that...

	// SwarmEnabled specifies whether the node should run the Swarm protocol.
	SwarmEnabled bool

	// SwarmAccount specifies account ID used for starting Swarm node.
	SwarmAccount string

	// SwarmAccountPassword specifies password for account retrieval from the keystore.
	SwarmAccountPassword string
}

// defaultNodeConfig contains the default node configuration values to use if all
// or some fields are missing from the user's specified list.
var defaultNodeConfig = &NodeConfig{
	MaxPeers:          25,
	EthereumEnabled:   false,
	EthereumNetworkID: 1278,
}

// NewNodeConfig creates a new node option set, initialized to the default values.
func NewNodeConfig() *NodeConfig {
	config := *defaultNodeConfig
	return &config
}

// Node represents a Geth Ethereum node instance.
type Node struct {
	node *node.Node
}

// gomobile不支持切片类型参数
type Bootnodes struct {
	Enodes []string
}

// getSwarmKey is a helper for finding and decrypting given account's private key
// to be used with Swarm.
func getSwarmKey(stack *node.Node, account, password string) (*keystore.Key, error) {
	if account == "" {
		return nil, nil // it's not an error, just skip
	}

	address := common.HexToAddress(account)
	acc := accounts.Account{
		Address: address,
	}

	am := stack.AccountManager()
	ks := am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	a, err := ks.Find(acc)
	if err != nil {
		return nil, fmt.Errorf("find account: %v", err)
	}
	keyjson, err := ioutil.ReadFile(a.URL.Path)
	if err != nil {
		return nil, fmt.Errorf("load key: %v", err)
	}

	return keystore.DecryptKey(keyjson, password)
}

func NewSwarmNode(datadir string, config *NodeConfig) (stack *Node, _ error) {
	if config == nil {
		config = NewNodeConfig()
	}
	if config.MaxPeers == 0 {
		config.MaxPeers = defaultNodeConfig.MaxPeers
	}
	/*
		if config.BootstrapNodes == nil || config.BootstrapNodes.Size() == 0 {
			config.BootstrapNodes = defaultNodeConfig.BootstrapNodes
		}
	*/

	// Create the empty networking stack
	nodeConf := &node.Config{
		Name: clientIdentifier,
		//Version:     params.VersionWithMeta,
		DataDir:     datadir,
		IPCPath:     "bzzd.ipc",
		KeyStoreDir: filepath.Join(datadir, "keystore"), // Mobile should never use internal keystores!
		P2P: p2p.Config{
			NoDiscovery: false,
			NoDial:      true,
			ListenAddr:  ":0",
			NAT:         nat.Any(),
			MaxPeers:    config.MaxPeers,
		},
	}

	rawStack, err := node.New(nodeConf)
	if err != nil {
		return nil, err
	}
	bzzconfig := bzzapi.NewConfig()
	bzzconfig.NetworkID = 1278
	key, err := getSwarmKey(rawStack, config.SwarmAccount, config.SwarmAccountPassword)
	if err != nil {
		return nil, fmt.Errorf("no key")
	}
	bzzconfig.Init(key.PrivateKey)

	if err := rawStack.Register(func(*node.ServiceContext) (node.Service, error) {
		return swarm.NewSwarm(bzzconfig, nil)
	}); err != nil {
		return nil, fmt.Errorf("swarm init: %v", err)
	}

	return &Node{rawStack}, nil
}

// Close terminates a running node along with all it's services, tearing internal
// state doen too. It's not possible to restart a closed node.
func (n *Node) Close() error {
	return n.node.Close()
}

// Start creates a live P2P node and starts running it.
func (n *Node) Start() error {
	return n.node.Start()
}

// Stop terminates a running node along with all it's services. If the node was
// not started, an error is returned.
func (n *Node) Stop() error {
	return n.node.Stop()
}

func (n *Node) Wait() {
	n.node.Wait()
}

func (n *Node) AddPeer(peer *Enodev4) {
	n.node.Server().AddPeer(peer.node)
}

func DefaultDataDir() string {
	return node.DefaultDataDir()
}

func (n *Node) AddBootnode(enode *Enodev4) {
	n.AddPeer(enode)
}

/*
func (n *Node) GetBootnodes(bootnodes *Bootnodes) *Enodesv4 {
	enodes := NewEnodesEmptyv4()
	for _, url := range bootnodes.Enodes {
		node, err := NewEnodev4(url)
		if err != nil {
			fmt.Println("Bootstrap URL invalid", "enode", url, "err", err)
		} else {
			enodes.Append(node)
		}
	}
	return enodes
}
*/
// GetNodeInfo gathers and returns a collection of metadata known about the host.
func (n *Node) GetNodeInfo() *NodeInfo {
	return &NodeInfo{n.node.Server().NodeInfo()}
}

// GetPeersInfo returns an array of metadata objects describing connected peers.
func (n *Node) GetPeersInfo() *PeerInfos {
	return &PeerInfos{n.node.Server().PeersInfo()}
}
