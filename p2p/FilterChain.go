package p2p

import (
	"github.com/plotozhu/MDCMainnet/p2p/discover"
	"github.com/plotozhu/MDCMainnet/p2p/enode"
)

type FilterChain struct {


	Filters   []discover.FiterItem


}

func NewFilterChain() *FilterChain {
	return &FilterChain{
		Filters:make([]discover.FiterItem,0),
	}
}

func (fc *FilterChain)AddFilter(ft discover.FiterItem){
	fc.Filters = append(fc.Filters,ft)
}

func (fc *FilterChain)IsBlocked(id enode.ID) bool{
	for _,ft := range fc.Filters {
		if ft.IsBlocked(id) {
			return true
		}
	}
	return false
}
func (fc *FilterChain)CanSendPing(id enode.ID) bool{
	if fc.IsBlocked(id) {
		return false
	}
	for _,ft := range fc.Filters {
		if !ft.CanSendPing(id) {
			return false
		}
	}
	return true
}
func (fc *FilterChain)CanProcPing(id enode.ID) bool{
	if fc.IsBlocked(id) {
		return false
	}
	for _,ft := range fc.Filters {
		if !ft.CanProcPing(id) {
			return false
		}
	}
	return true
}
func (fc *FilterChain)CanProcPong(id enode.ID) bool{
	if fc.IsBlocked(id) {
		return false
	}
	for _,ft := range fc.Filters {
		if ft.CanProcPong(id) {
			return false
		}
	}
	return true
}
func (fc *FilterChain)CanStartConnect(id enode.ID) bool{
	if fc.IsBlocked(id) {
		return false
	}
	for _,ft := range fc.Filters {
		if !ft.CanStartConnect(id) {
			return false
		}
	}
	return true
}
func (fc *FilterChain)ShouldAcceptConn(id enode.ID) bool{
	if fc.IsBlocked(id) {
		return false
	}
	for _,ft := range fc.Filters {
		if !ft.ShouldAcceptConn(id) {
			return false
		}
	}
	return true
}