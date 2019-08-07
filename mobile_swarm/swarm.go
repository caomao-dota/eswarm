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

package csdc

//import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/plotozhu/MDCMainnet/accounts"
	"github.com/plotozhu/MDCMainnet/accounts/keystore"
	"github.com/plotozhu/MDCMainnet/common"
	"github.com/plotozhu/MDCMainnet/node"
	"github.com/plotozhu/MDCMainnet/p2p"
	"github.com/plotozhu/MDCMainnet/p2p/nat"
	"github.com/plotozhu/MDCMainnet/swarm"
	bzzapi "github.com/plotozhu/MDCMainnet/swarm/api"
	"github.com/plotozhu/MDCMainnet/swarm/util"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

	ServerAddr string
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
	node     *node.Node
	httpPort string
}

type ActivatePost struct {
	Appid      string
	Credential string
	Account    string
	ClientId   string
}

type RespData struct {
	ExpireTime int64
	Error      string
}

func PostToServer(urlStr string, timeout time.Duration, data *ActivatePost) (int64, error) {

	client := &http.Client{
		Timeout: timeout * time.Second,
	}

	postdata := make(url.Values)
	postdata.Set("appId", data.Appid)
	postdata.Set("account", data.Account)
	postdata.Set("credential", data.Credential)
	postdata.Set("clientId", data.ClientId)
	resp, err := client.PostForm(urlStr, postdata)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	respData := RespData{}
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("resp.Body: %v", respData.Error)
	}

	return respData.ExpireTime, nil
}

func Start(path string, password string, bootnodeAddrs string, bootnode string) (stack *Node, _ error) {
	//path keystore上一级目录
	if path == "" {
		return nil, errors.New("Must input path ...")
	}

	if password == "" {
		password = "123"
	}

	var account string
	fileinfo, err := ioutil.ReadDir(path + "/keystore")
	if err != nil && len(fileinfo) == 0 {
		return nil, errors.New("Please input the correct directory or keystore dir is empty. Please activate first...")
	}
	for _, file := range fileinfo {
		if !file.IsDir() { //UTC--2019-03-26T10-26-19.431075000Z--3de7c66e10c0f476c9b0d221e9cf1affd50eeac2
			account = file.Name()
			account = account[len(account)-40:]
			break
		}
	}

	bootnodes := []string{}

	config := NewNodeConfig()
	config.SwarmAccount = account
	config.SwarmAccountPassword = password

	if bootnodeAddrs != "" {
		nodes, reportUrl, err := util.GetBootnodesInfo(bootnodeAddrs)
		if err == nil {
			if len(nodes) != 0 {
				bootnodes = append(bootnodes, nodes...)
			}
		}
		config.ServerAddr = reportUrl
	}

	stack, err = NewSwarmNode(path, config)
	if err != nil {
		return nil, errors.New("NewSwarmNode func err...")
	}

	if swarmErr := stack.Start(); swarmErr != nil {
		return nil, swarmErr
	}

	stack.GetNodeInfo()

	if bootnode != "" {
		bootnodes = append(bootnodes, bootnode)

	}

	for _, boot := range bootnodes {
		stack.AddBootnode(boot)
	}

	return stack, nil
}

func CreateKeyStore(path, passwd string, ScryptN, ScryptP int) (string, error) {
	keystore := NewKeyStore(path, ScryptN, ScryptP)
	account, err := keystore.NewAccount(passwd)
	if err != nil {
		return "", errors.New("keystore.NewAccount func err...")
	}
	return account.GetAddress().GetHex(), nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func Activate(path string, appId string, credential string, addr string, newAccount bool, password string, arg int) (int64, error) {
	return ActivateR(path, appId, "", credential, addr, newAccount, password, arg)
}

func ActivateR(path string, appId string, clientId string, credential string, addr string, newAccount bool, password string, arg int) (int64, error) {

	if path == "" {
		return 0, errors.New("Must input path ...")
	}

	//清除缓存
	err := Clean(path, arg)
	if err != nil {
		return 0, errors.New("Clean cache fail ...")
	}

	if addr == "" {
		addr = "https://service.cdscfund.org/apis/v1/activate"
	}

	if password == "" {
		password = "123"
	}

	keystorepath := path + "/keystore/"
	exist, err := PathExists(keystorepath)
	if err != nil {
		return 0, err
	}
	if !exist {
		err = os.Mkdir(path+"/keystore", os.ModePerm)
		if err != nil {
			return 0, err
		}
	}

	bzzAccount := ""
	fileinfo, err := ioutil.ReadDir(keystorepath)
	if err != nil {
		return 0, err
	}
	for _, file := range fileinfo {
		if !file.IsDir() {
			//UTC--2019-03-26T10-26-19.431075000Z--3de7c66e10c0f476c9b0d221e9cf1affd50eeac2
			if newAccount == false {
				bzzAccount = file.Name()
				bzzAccount = bzzAccount[len(bzzAccount)-40:]
				bzzAccount = "0x" + bzzAccount
			} else { //重新生成一个keystore
				err := os.Remove(keystorepath + file.Name())
				if err != nil {
					return 0, err
				}
			}

			break
		}
	}

	if bzzAccount == "" {
		account, err := CreateKeyStore(keystorepath, password, StandardScryptN, StandardScryptP)
		if err != nil {
			return 0, err
		}
		bzzAccount = account
	}

	activatePost := &ActivatePost{appId, credential, bzzAccount, clientId}
	ti, err := PostToServer(addr, 5, activatePost)
	if err != nil {
		return 0, err
	}
	return ti, err
}

func Clean(path string, arg int) error {
	//path keystore上一级目录
	//arg 0:删除path下所有，1：删除bzz缓存，2：删除SwarmMobile目录，3：删除SwarmMobile下nodes目录
	if path == "" {
		return errors.New("Must input path or platform（android or ios）...")
	}
	exist, err := PathExists(path)
	if err != nil {
		return nil
	}
	if !exist {
		return nil
	}

	if arg == 2 {
		err := os.RemoveAll(path + "SwarmMobile")
		if err != nil {
			return fmt.Errorf("Clear fail: %v", err)
		}
		return nil
	} else if arg == 3 {
		err := os.RemoveAll(path + "SwarmMobile/nodes")
		if err != nil {
			return fmt.Errorf("Clear fail: %v", err)
		}
		return nil
	}

	fileinfo, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range fileinfo {
		if arg == 0 {
			err := os.RemoveAll(path + file.Name())
			if err != nil {
				return fmt.Errorf("Clear fail: %v", err)
			}
		} else if arg == 1 {
			if len(file.Name()) == 44 && file.Name()[:4] == "bzz-" {
				err := os.RemoveAll(path + file.Name())
				if err != nil {
					return fmt.Errorf("Clear fail: %v", err)
				}
			}
		}
	}

	return nil

}

// getSwarmKey is a helper for finding and decrypting given account's private key
// to be used with Swarm.
func getSwarmKey(stack *node.Node, account string, password string) (*keystore.Key, error) {
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

	// Create the empty networking stack
	nodeConf := &node.Config{
		Name: clientIdentifier,
		//Version:     params.VersionWithMeta,
		DataDir: datadir,
		//IPCPath:     "bzzd.ipc",
		KeyStoreDir: filepath.Join(datadir, "keystore"), // Mobile should never use internal keystores!
		P2P: p2p.Config{
			NoDiscovery: false,
			NoDial:      true,
			ListenAddr:  ":0",
			NAT:         nat.Any(),
			MaxPeers:    config.MaxPeers,
			NodeType:    17,
		},
		NoUSB: true,
	}

	rawStack, err := node.New(nodeConf)
	if err != nil {
		return nil, err
	}
	bzzconfig := bzzapi.NewConfig()

	// start swarm http proxy server
	l, _ := net.Listen("tcp", ":0") // listen on localhost
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	bzzconfig.Port = strconv.Itoa(port)

	bzzconfig.Path = datadir
	bzzconfig.NodeType = 17
	bzzconfig.LocalStoreParams.DbCapacity = 20000  //5G
	bzzconfig.LocalStoreParams.CacheCapacity = 100 //25M
	if config.ServerAddr != "" {
		bzzconfig.ServerAddr = config.ServerAddr
	}
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

	return &Node{rawStack, bzzconfig.Port}, nil
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

func (n *Node) AddPeer(peer *Enodev4) {
	n.node.Server().AddPeer(peer.node)
}

func DefaultDataDir() string {
	return node.DefaultDataDir()
}

func (n *Node) AddBootnode(enodeStr string) error {

	enode, err := NewEnodev4(enodeStr)
	if err != nil {
		return err
	}
	n.AddPeer(enode)
	return nil
}

// GetNodeInfo gathers and returns a collection of metadata known about the host.
func (n *Node) GetNodeInfo() *NodeInfo {
	return &NodeInfo{n.node.Server().NodeInfo()}
}

// GetPeersInfo returns an array of metadata objects describing connected peers.
func (n *Node) GetPeersInfo() *PeerInfos {
	return &PeerInfos{n.node.Server().PeersInfo()}
}

// getBzzPort
func (n *Node) GetHttpPort() string {
	return n.httpPort
}

// getM3U8Url
func (n *Node) GetM3U8BaseUrl() string {

	return fmt.Sprintf("http://localhost:%v/m3u8:/", n.httpPort)
}

// GetNodeInfo gathers and returns a collection of metadata known about the host.
func (n *Node) GetM3U8Url(cdnUrl string, hash string) string {

	index := strings.LastIndex(cdnUrl, "/")
	baseUrl := n.GetM3U8BaseUrl()

	url := fmt.Sprintf("%v/{%v}%v", cdnUrl[:index], hash, cdnUrl[index:])
	url = strings.Replace(url, "://", "/", -1)

	return fmt.Sprintf("%v%v", baseUrl, url)
}
