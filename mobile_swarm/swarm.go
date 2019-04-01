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

package eswarm

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
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

type ActivatePost struct {
	Appid      string
	Credential string
	Account    string
}

type RespData struct {
	ExpireTime int
}

func PostToServer(urlstr string, timeout time.Duration, data *ActivatePost) int {

	postdata := make(url.Values)
	postdata.Set("appId", data.Appid)
	postdata.Set("account", data.Account)
	postdata.Set("credential", data.Credential)
	resp, err := http.PostForm(urlstr, postdata)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0
	}
	respData := RespData{}
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return 0
	}
	return respData.ExpireTime
}

/*
func SwarmStart1(path string, options interface{}) error {
	//path keystore上一级目录
	if path == "" {
		return errors.New("Must input path...")
	}

	var nodestr, account string
	switch ins := options.(type){
	case StartOptions:
		log.Info("%t, %v, %s",ins,ins,ins.Bootnode)
		nodestr = ins.Bootnode
	case *StartOptions:
		log.Info("%t, %v, %s",ins,ins,ins.Bootnode)
		nodestr = ins.Bootnode
	case nil:
		nodestr = "enode://32136bcf7097861d9438a0d2923846a118361e4a7181b2ce863a45f435c8ebc98708edf865a470d2678d064273f6b6d2bf6c2d9321100d0718a576ad0630c288@172.16.1.10:30399"
	default:
		return errors.New("Please input the correct type BootNode or *BootNode...")
	}

	fileinfos, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println("Please input the correct directory...")
		return err
	}
	flag := true
	for _, files := range fileinfos {
		if files.IsDir() && files.Name() == "keystore" {
			flag = false
			fileinfo, err := ioutil.ReadDir(path + "/keystore")
			if err != nil { fmt.Println("Please input the correct directory..."); return err}
			for _, file := range fileinfo {
				if !file.IsDir() { //UTC--2019-03-26T10-26-19.431075000Z--3de7c66e10c0f476c9b0d221e9cf1affd50eeac2
					account = file.Name()
					account = account[len(account) - 40:]
					break
				}
			}
		}
		if flag == false {
			break
		}
	}
	if flag == true {
		//没有keystore目录
		return errors.New("There is no keystore directory in the current directory...")
	}

	config := NewNodeConfig()

	config.SwarmAccount = account
	config.SwarmAccountPassword = "123"
	stack,err := NewSwarmNode(path, config)
	if err != nil{ return errors.New("NewSwarmNode func err...")}
	stack.Start()

	//每个bootnode都NewEnodev4，AddBootnode
	enode ,err := NewEnodev4(nodestr)
	if err != nil { return errors.New("NewEnodev4 func err...")}
	stack.AddBootnode(enode)


	return nil
}
*/

func SwarmStart(path string, password string, bootnode string) (stack *Node, _ error) {
	//path keystore上一级目录
	if path == "" {
		return nil, errors.New("Must input path ...")
	}

	if password == "" {
		password = "123"
	}

	/*
		case StartOptions:
			log.Info("%t, %v, %s", ins, ins, ins.Bootnode)
			if ins.BzzPassword == "" {
				passwd = "123"
			} else {
				passwd = ins.BzzPassword
			}
			if len(ins.Bootnode) == 0 {
				enode, err := NewEnodev4(DefaultBootNode)
				if err != nil {
					return errors.New("NewEnodev4 func err...")
				}
				enodes = append(enodes, enode)
			} else {
				for _, str := range ins.Bootnode {
					enode, err := NewEnodev4(str)
					if err != nil {
						return errors.New("NewEnodev4 func err...")
					}
					enodes = append(enodes, enode)
				}
			}
	*/

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

	config := NewNodeConfig()
	config.SwarmAccount = account
	config.SwarmAccountPassword = password
	stack, err = NewSwarmNode(path, config)
	if err != nil {
		return nil, errors.New("NewSwarmNode func err...")
	}
	stack.Start()

	if bootnode != "" {
		err := stack.AddBootnode(bootnode)
		if err != nil {
			stack.Close()
			return nil, err
		}
	}

	return stack, nil
}

/*
func SwarmStart1(path string, options interface{}) error {
	//path keystore上一级目录
	if path == "" {
		return errors.New("Must input path...")
	}
	DefaultBootNode := "enode://60baeb93a60e809141133538431aace2cee7e220b36a57684c7ad5cc5f97d7adc6c2409eb73acdc46ed98a828266eb79545a12a9b36ffc830a65de9cb336760d@172.16.1.10:30399"

	enodes := make([]*Enodev4, 0)
	var account, passwd string
	switch ins := options.(type) {
	case StartOptions:
		log.Info("%t, %v, %s", ins, ins, ins.Bootnode)
		if ins.BzzPassword == "" {
			passwd = "123"
		} else {
			passwd = ins.BzzPassword
		}
		if len(ins.Bootnode) == 0 {
			enode, err := NewEnodev4(DefaultBootNode)
			if err != nil {
				return errors.New("NewEnodev4 func err...")
			}
			enodes = append(enodes, enode)
		} else {
			for _, str := range ins.Bootnode {
				enode, err := NewEnodev4(str)
				if err != nil {
					return errors.New("NewEnodev4 func err...")
				}
				enodes = append(enodes, enode)
			}
		}
	case *StartOptions:
		log.Info("%t, %v, %s", ins, ins, ins.Bootnode)
		if ins.BzzPassword == "" {
			passwd = "123"
		} else {
			passwd = ins.BzzPassword
		}
		if len(ins.Bootnode) == 0 {
			enode, err := NewEnodev4(DefaultBootNode)
			if err != nil {
				return errors.New("NewEnodev4 func err...")
			}
			enodes = append(enodes, enode)
		} else {
			for _, str := range ins.Bootnode {
				enode, err := NewEnodev4(str)
				if err != nil {
					return errors.New("NewEnodev4 func err...")
				}
				enodes = append(enodes, enode)
			}
		}
	case nil:
		passwd = "123"
		enode, err := NewEnodev4(DefaultBootNode)
		if err != nil {
			return errors.New("NewEnodev4 func err...")
		}
		enodes = append(enodes, enode)
	default:
		return errors.New("Please input the correct type BootNode or *BootNode...")
	}

	fileinfo, err := ioutil.ReadDir(path + "/keystore")
	if err != nil && len(fileinfo) == 0 {
		return errors.New("Please input the correct directory or keystore dir is empty. Please activate first...")
	}
	for _, file := range fileinfo {
		if !file.IsDir() { //UTC--2019-03-26T10-26-19.431075000Z--3de7c66e10c0f476c9b0d221e9cf1affd50eeac2
			account = file.Name()
			account = account[len(account)-40:]
			break
		}
	}

	config := NewNodeConfig()
	config.SwarmAccount = account
	config.SwarmAccountPassword = passwd
	stack, err := NewSwarmNode(path, config)
	if err != nil {
		return errors.New("NewSwarmNode func err...")
	}
	stack.Start()

	for _, enode := range enodes {
		stack.AddBootnode(enode)
	}

	return nil
}
*/

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

func Activate(path string, appId string, credential string, addr string, newAccount bool, password string) int {
	if path == "" {
		return 0
	}

	if addr == "" {
		addr = "http://172.16.1.10:4000/apis/v1/activate"
	}

	if password == "" {
		password = "123"
	}

	keystorepath := path + "/keystore/"
	exist, err := PathExists(keystorepath)
	if err != nil {
		return 0
	}
	if !exist {
		err = os.Mkdir(path+"/keystore", os.ModePerm)
		if err != nil {
			return 0
		}
	}

	bzzAccount := ""
	fileinfo, err := ioutil.ReadDir(keystorepath)
	if err != nil {
		return 0
	}
	for _, file := range fileinfo {
		if !file.IsDir() {
			//UTC--2019-03-26T10-26-19.431075000Z--3de7c66e10c0f476c9b0d221e9cf1affd50eeac2
			if newAccount == false {
				bzzAccount = file.Name()
				bzzAccount = bzzAccount[len(bzzAccount)-40:]
			} else { //重新生成一个keystore
				err := os.Remove(keystorepath + file.Name())
				if err != nil {
					return 0
				}
			}

			break
		}
	}

	if bzzAccount == "" {
		account, err := CreateKeyStore(keystorepath, password, StandardScryptN, StandardScryptP)
		if err != nil {
			return 0
		}
		bzzAccount = account
	}

	/*
		switch ins := options.(type) {
		case ActivateOptions:
			if ins.NewAccount == true { //重新生成一个keystore
				fileinfo, err := ioutil.ReadDir(keystorepath)
				if err != nil {
					return err
				}
				for _, file := range fileinfo {
					if !file.IsDir() { //UTC--2019-03-26T10-26-19.431075000Z--3de7c66e10c0f476c9b0d221e9cf1affd50eeac2
						err := os.Remove(keystorepath + file.Name())
						if err != nil {
							return errors.New("os.Remove(file.Name()) err. delete keystore file fail...")
						}
						break
					}
				}
			}

			fileinfo, err := ioutil.ReadDir(keystorepath)
			if err != nil {
				return errors.New("Please input the correct directory...")
			}

			if len(fileinfo) == 0 {
				err := CreateKeyStore(keystorepath, ins.BzzPassword, StandardScryptN, StandardScryptP)
				if err != nil {
					return errors.New("CreateKeyStore func err...")
				}
			}

		case *ActivateOptions:
			if ins.NewAccount == true { //重新生成一个keystore
				err := os.RemoveAll(keystorepath)
				if err != nil {
					return errors.New("os.RemoveAll err...")
				}
			}

			fileinfo, err := ioutil.ReadDir(keystorepath)
			if err != nil {
				return errors.New("Please input the correct directory...")
			}

			if len(fileinfo) == 0 {
				err := CreateKeyStore(keystorepath, ins.BzzPassword, StandardScryptN, StandardScryptP)
				if err != nil {
					return errors.New("CreateKeyStore func err...")
				}
			}

		case nil:
			fileinfo, err := ioutil.ReadDir(keystorepath)
			if err != nil {
				return errors.New("Please input the correct directory...")
			}
			if len(fileinfo) == 0 {
				err := CreateKeyStore(keystorepath, "123", StandardScryptN, StandardScryptP)
				if err != nil {
					return errors.New("CreateKeyStore func err...")
				}
			}
	*/

	activatePost := &ActivatePost{appId, credential, bzzAccount}
	ti := PostToServer(addr, time.Second, activatePost)
	return ti
}

func Clean(path string, all bool) error {
	//path keystore上一级目录
	if path == "" {
		return errors.New("Must input path...")
	}
	exist, err := PathExists(path)
	if err != nil {
		return err
	}
	if !exist {
		return nil
	}

	fileinfo, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	for _, file := range fileinfo {
		if all {
			err := os.RemoveAll(path + file.Name())
			if err != nil {
				return fmt.Errorf("Clear fail: %v", err)
			}
		} else {
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

/*
func Activate1(path, appid, credential string, options interface{}) error {
	if path == "" {
		return errors.New("Must input path...")
	}
	keystorepath := path + "/keystore/"
	exist, err := PathExists(keystorepath)
	if err != nil {
		return errors.New("Get dir error...")
	}
	if !exist {
		err = os.Mkdir(path+"/keystore", os.ModePerm)
		if err != nil {
			return errors.New("os.Mkdir keystore fail...")
		}
	}

	appid = ""
	credential = ""
	switch ins := options.(type) {
	case ActivateOptions:
		if ins.NewAccount == true { //重新生成一个keystore
			fileinfo, err := ioutil.ReadDir(keystorepath)
			if err != nil {
				return err
			}
			for _, file := range fileinfo {
				if !file.IsDir() { //UTC--2019-03-26T10-26-19.431075000Z--3de7c66e10c0f476c9b0d221e9cf1affd50eeac2
					err := os.Remove(keystorepath + file.Name())
					if err != nil {
						return errors.New("os.Remove(file.Name()) err. delete keystore file fail...")
					}
					break
				}
			}
		}

		fileinfo, err := ioutil.ReadDir(keystorepath)
		if err != nil {
			return errors.New("Please input the correct directory...")
		}

		if len(fileinfo) == 0 {
			err := CreateKeyStore(keystorepath, ins.BzzPassword, StandardScryptN, StandardScryptP)
			if err != nil {
				return errors.New("CreateKeyStore func err...")
			}
		}

	case *ActivateOptions:
		if ins.NewAccount == true { //重新生成一个keystore
			err := os.RemoveAll(keystorepath)
			if err != nil {
				return errors.New("os.RemoveAll err...")
			}
		}

		fileinfo, err := ioutil.ReadDir(keystorepath)
		if err != nil {
			return errors.New("Please input the correct directory...")
		}

		if len(fileinfo) == 0 {
			err := CreateKeyStore(keystorepath, ins.BzzPassword, StandardScryptN, StandardScryptP)
			if err != nil {
				return errors.New("CreateKeyStore func err...")
			}
		}

	case nil:
		fileinfo, err := ioutil.ReadDir(keystorepath)
		if err != nil {
			return errors.New("Please input the correct directory...")
		}
		if len(fileinfo) == 0 {
			err := CreateKeyStore(keystorepath, "123", StandardScryptN, StandardScryptP)
			if err != nil {
				return errors.New("CreateKeyStore func err...")
			}
		}
	}
	return nil
}
*/

/*
func Activate(path, appId, clientId, credential string, options *OptionsArgs) (int, error) {
	data := &ActivatePost{appId,clientId,credential}
	PostToServer("www.baidu.com", data)
	if path == "" {
		return 0, errors.New("Must input path...")
	}


	return 0,nil
}
*/

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
	/*
		if config.BootstrapNodes == nil || config.BootstrapNodes.Size() == 0 {
			config.BootstrapNodes = defaultNodeConfig.BootstrapNodes
		}
	*/

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
		},
		NoUSB: true,
	}

	rawStack, err := node.New(nodeConf)
	if err != nil {
		return nil, err
	}
	bzzconfig := bzzapi.NewConfig()
	bzzconfig.LightNodeEnabled = true
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

/*
func (n *Node) Wait() {
	n.node.Wait()
}
*/

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
