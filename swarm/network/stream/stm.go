package stream

import (
	"context"
	"github.com/plotozhu/MDCMainnet/log"
	"github.com/plotozhu/MDCMainnet/p2p/enode"
	"github.com/plotozhu/MDCMainnet/swarm/network"
	"sync"
	"time"
)

/***
	这是用于流同步的状态机，详细的设计方案见文档《流的同步》
 */
type STATES uint8
type EVENT  uint8
const (
	StateInitS STATES = iota
	StateS1		//发送SubscriptionRequest,还没有收到Subscribe
	StateS2		//SYNC 收到了SubScribe
	StateS3		//RETRIEVAL 收到了SubScribe
	StateInitC
	StateC1     //发送了Subscribe,没有收到OfferedHashes
	StateC2
	StateC3
	StateC4
	StateCnt
)

type holdInfo struct {
	r *Registry
	p *network.Peer
	bin uint8
	subs map[enode.ID]map[Stream]struct{}
}

type clientInfo struct {
	peerId enode.ID
	s Stream
	h *Range
	priority uint8
}
type stm struct {
	state   STATES
	ticker  int //超时值，以100ms为单位，每隔100ms系统就会调用一次

	peer   *Peer
	stream Stream
	history  *Range
	priority uint8
	lastOfferedHashes *OfferedHashesMsg
	enterFuncs [StateCnt]EnterStateFunc
	mutex sync.Mutex

}
type ClientHolder interface{
	DoSubscribe(	s Stream,h *Range, priority uint8)
	ProcOfferedHashes(c* client,ctx context.Context, req *OfferedHashesMsg) (error,int)
}
type Holder interface {
	DoRequestSubscription(info *holdInfo)
	ProcessOfferedHashes(info holdInfo)

}
type  EnterStateFunc func()

//创建一个服务端的STM
func CreateServerStm(p *Peer,s Stream, h *Range,prio uint8) *stm{
	stmInst := stm{
		state:StateInitS,
		ticker:0,
		peer:p,
		stream:s,
		history:h,
		priority:prio,


	}
	stmInst.enterFuncs = [StateCnt]EnterStateFunc{
		nil,
		stmInst.enterS1,
		stmInst.enterS2,
		stmInst.enterS2,//use this to clear timer
		nil,
	}
	return &stmInst
}
func CreateClientStm(p *Peer,s Stream, h *Range,prio uint8)*stm{
	stmInst := stm{
		state:StateInitC,
		ticker:0,
		peer:p,
		stream:s,
		history:h,
		priority:prio,

	}
	stmInst.enterFuncs = [StateCnt]EnterStateFunc{
		nil,
		nil,
		nil,
		nil,
		nil,
		stmInst.enterC1,
		stmInst.enterC2,
		stmInst.enterC3,
		stmInst.enterC4,
	}
	return &stmInst
}
func (s *stm )enterS1() {
	s.setTimeout(5000)
	//发送subscribeRequest
	s.peer.DoRequestSubscription(s.stream,s.history,s.priority)
}

func (s *stm )enterS2() {
	s.clearTimeout()
	//发送subscribeRequest

}

func (s *stm)enterC1(){

	s.peer.DoSubscribe(s.stream,s.history,s.priority)
	if s.stream.Name != swarmChunkServerStreamName {
		s.setTimeout(5000)
	}
}

func (s *stm)enterC2(){
	s.setTimeout(5000)

}
func (s *stm)enterC3(){
	s.clearTimeout()
}
func (s *stm)enterC4(){
	s.setTimeout(20000)
	//TODO checkOfferedHashes
}
/**
设置定时器，以ms为单位，实际的精度为100ms
 */
func (s *stm)setTimeout(timeout int){
	s.mutex.Lock()
	s.ticker = timeout/100
	s.mutex.Unlock()
}
func (s *stm)clearTimeout(){
	s.mutex.Lock()
	s.ticker = 0
	s.mutex.Unlock()
}
func (s *stm)EnterState(newState STATES ){
	log.Info("Enter State:","stream",s.stream,"cur",s.state,"new",newState)


	s.state = newState
	if s.enterFuncs[newState] != nil {
		go s.enterFuncs[newState]()
	}

}

func (s *stm)OnOfferedHashes(req *OfferedHashesMsg) error {
	s.lastOfferedHashes = req
	switch s.state {
	case StateC1:
		fallthrough
	case StateC2:
		fallthrough
	case StateC4:
		s.doProcOfferedHashes()
	default:
		log.Warn(" OfferedHashes on wrong state:","stream",s.stream.String(),"state",s.state)
	}
	return nil
}

func (s *stm)OnWantedHashes(req *WantedHashesMsg){
	switch s.state {
	case StateS1:
		fallthrough
	case StateS2:
		ctx,_ := context.WithTimeout(context.TODO(),30*time.Second)
		s.peer.handleWantedHashesMsg(ctx,req)

	default:
		log.Warn(" wanted hashes on wrong state:","stream",s.stream.String(),"state",s.state)
	}
}

func (s *stm)OnSubscribe(req *SubscribeMsg){
	s.history = req.History
	log.Info("Subscription received","stream",req.Stream.String())
	switch s.state {
	case StateS1:
		if s.stream.Name == swarmChunkServerStreamName {
			s.EnterState(StateS3)
		}else{
			var from,to uint64;
			if s.history == nil {
				from = 0
				to = 0
			}else {
				from = s.history.From
				to   = s.history.To
			}

			s.EnterState(StateS2)
			go s.peer.SendOfferedHashes(s.peer.servers[s.stream],from,to)
		}

	default:
		log.Warn(" Subscription on wrong state:","stream",req.Stream.String(),"state",s.state)

		//s.EnterState(StateS2)

	}

}
func (s *stm)OnSubscriptionReq(from,to uint64){
	s.history = NewRange(from,to)
	switch s.state {
	case StateInitC:
		s.EnterState(StateC1)
	case StateC1:
		s.peer.DoSubscribe(s.stream,s.history,s.priority);
	default:
		log.Warn(" SubscriptionRequest on wrong state:","stream",s.stream.String(),"state",s.state)
	}
}
func (s *stm)onTimeout(){
	switch s.state {
	case StateS1:
		s.EnterState(StateS1)//重新进入，调用一次进入函数

	case StateC1:
		s.EnterState(StateC1) //重新进入，调用一次进入函数
	case StateC2:
		s.doProcOfferedHashes()
	case StateC4:
		s.doProcOfferedHashes()

	}
}
func (s *stm)doProcOfferedHashes(){
	go func(){
		ctx,_ := context.WithTimeout(context.TODO(),30*time.Second)
		if s.peer != nil && s.peer.clients != nil  {
			if s.lastOfferedHashes != nil {
				client := s.peer.clients[s.stream]
				err,finished :=s.peer.procOfferedHashes(client,ctx,s.lastOfferedHashes)
				if err == nil {
					if finished {
						if s.stream.Live {
							s.EnterState(StateC4)
						}else {
							s.EnterState(StateC3)
						}
					}else{
						s.EnterState(StateC2)
					}
					return
				}
				log.Error(" ProcOfferedHashes error","id",s.peer.ID(),"Stream",s.stream.Name,"error",err)
				return ;
			}
			log.Error(" Proc OfferedHashes: offeredhashes not exist","id",s.peer.ID(),"Stream",s.stream.Name)


		}else {
			log.Error(" ProcOfferedHashes error peer or stream is null")
		}
	}()


}
//定时器处理，每100ms被调用一次
func (s *stm)OnTicker(){
	s.mutex.Lock()
	if s.ticker > 0 {
		s.ticker --
		if s.ticker == 0 {
			s.onTimeout()
		}
	}
	s.mutex.Unlock()
}