package storage

import (
	"fmt"
	"github.com/plotozhu/MDCMainnet/log"
	"github.com/plotozhu/MDCMainnet/swarm/chunk"
	"github.com/plotozhu/wiredtiger-go/wiredtiger"
	"math"
	"sync"
)
type TYPE_ENUM  uint8
const (
	T_QUERY TYPE_ENUM= 0  //查询
	T_UPDATE TYPE_ENUM= 1 //更新
	T_DELETE TYPE_ENUM=2  //删除
)
type requestItem struct {
	_type TYPE_ENUM
	address chunk.Address
	data    []byte
	ret     chan *resultItem
}

type resultItem struct {
	err   error
	data    []byte
}

type oneShard struct {
	session *wiredtiger.Session
	cursor  *wiredtiger.Cursor
	inputChan   chan *requestItem
	lock    sync.Mutex
}
type WiredtigerDB struct {
	path string
	shardMasks int
	shardsCount int
	conn  *wiredtiger.Connection
	shardItems []*oneShard
	quitC  chan struct{}
}
func NewDatabase(filePath string,shardOverlay int) *WiredtigerDB {
	shardCount := int(math.Pow(2,float64(shardOverlay)))
	db := WiredtigerDB{
		path:filePath,
		shardsCount:shardCount,
		shardMasks:int(shardCount-1),
		shardItems:make([]*oneShard,shardCount),
	}

	db.OpenDB();

	return &db
}

func (db *WiredtigerDB)OpenDB(){
	if db.conn != nil {
		db.conn.Close("")
	}
	var err error
	db.conn,err = wiredtiger.Open(db.path,"create")

	if err != nil {
		log.Error(err.Error())
	}

	for i := 0; i < db.shardsCount;i++ {

		session, err := db.conn.OpenSession("")
		if err != nil {

			panic(fmt.Sprintf("Failed to create session: %v", err.Error()))

		}


		err = session.Create(fmt.Sprintf("table:rawchunks%d",i), "key_format=u,value_format=u")
		if err != nil {
			panic(fmt.Sprintf("Failed to open cursor table: %v", err.Error()))
		}
		cursor, err := session.OpenCursor(fmt.Sprintf("table:rawchunks%d",i), nil, "")

		db.shardItems[i] = &oneShard{
			session:session,
			cursor:cursor,
			inputChan:make(chan *requestItem),
		}
	}
	db.start();
}

func (db *WiredtigerDB)start(){
	db.quitC = make(chan struct{})
	for i := 0; i < db.shardsCount; i++{
		go func (index int ){
			for {
				select {
					case item := <- db.shardItems[index].inputChan:
						db.procRequest(db.shardItems[index],item)
					case <- db.quitC:
							return
				}
			}
		}(i)
	}
}

func (db *WiredtigerDB) Close(){
	close(db.quitC)
	for i := 0; i < db.shardsCount; i++{
		if db.shardItems[i] != nil {
			db.shardItems[i].session.Close("")
		}
	}
}

func (db *WiredtigerDB)procRequest(shardItem *oneShard,request *requestItem){
	shardItem.lock.Lock()
	defer shardItem.lock.Unlock()
	switch request._type {
	case T_QUERY:
		value := make([]byte, 0)
		shardItem.cursor.SetKey([]byte(request.address[:]))
		err := shardItem.cursor.Search()
		if err == nil {
			err = shardItem.cursor.GetValue(&value)
		}
		if err != nil {
			log.Error("Failed to lookup", "addr", request.address, "error", err.Error())
		}

		request.ret <- &resultItem{err,value}
	case T_UPDATE:
		key := request.address

		err := shardItem.cursor.SetKey([]byte(key[:]))
		if err != nil {
			log.Error("error in set key", "reason", err)
		}
		err = shardItem.cursor.SetValue(encodeData(NewChunk(request.address,request.data)))
		if err != nil {
			err = shardItem.cursor.Update()
			if err != nil {
				log.Error("error in set data", "reason", err)
			}

		}
		err = shardItem.cursor.Insert()
		if err != nil {
			log.Error("Failed to insert", "error", err)
		} else {

			//			log.Info("Ok to insert","addr",chunk.Address(),"value",chunk.Data()[:10])
		} //<- s.waitChan
		//	}()

		request.ret <- &resultItem{err,[]byte{}}
	case T_DELETE:
		shardItem.cursor.SetKey([]byte(request.address))
		err := shardItem.cursor.Remove()
		if err != nil {
			log.Error("Failed to delete", "error", err.Error())
		} else {
			log.Info("chunk deleted:", "addr", request.address)

		}

		request.ret <- &resultItem{err,[]byte{}}
	}
}


// newMockEncodeDataFunc returns a function that stores the chunk data
// to a mock store to bypass the default functionality encodeData.
// The constructed function always returns the nil data, as DbStore does
// not need to store the data, but still need to create the index.
func (db *WiredtigerDB)NewWtEncodeDataFunc() func(chunk chunk.Chunk) []byte {


	return func(chunk chunk.Chunk) []byte {
		shardId := int(chunk.Address()[0]) & db.shardMasks
		shardItem := db.shardItems[shardId]
		result :=make(chan *resultItem)
		req := requestItem{T_UPDATE,chunk.Address(),chunk.Data(),result}
		shardItem.inputChan <- &req
		<- result
		return chunk.Address()[:]
	}
}
type resultV struct {
	data []byte
	err error
}
func (db *WiredtigerDB)NewWtGetDataFunc() func(addr chunk.Address) (data []byte, err error) {


	return func(addr chunk.Address) (data []byte, err error) {

		shardId := int(addr[0]) & db.shardMasks
		shardItem := db.shardItems[shardId]
		result :=make(chan *resultItem)
		req := requestItem{T_QUERY,addr,[]byte{},result}
		shardItem.inputChan <- &req
		ret := <- result
		return ret.data,ret.err

	}
}

func (db *WiredtigerDB)NewWtDeleteDataFunc() func(addr chunk.Address) (err error) {
	//创建一个删除线程
	return func(addr chunk.Address) (err error) {
		shardId := int(addr[0]) & db.shardMasks
		shardItem := db.shardItems[shardId]
		result :=make(chan *resultItem)
		req := requestItem{T_DELETE,addr,[]byte{},result}
		shardItem.inputChan <- &req
		ret := <- result
		return ret.err
	}
}