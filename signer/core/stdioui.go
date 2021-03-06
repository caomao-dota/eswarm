// Copyright 2018 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.
//

package core

import (
	"context"
	"sync"

	"github.com/gauss-project/eswarm/internal/ethapi"
	"github.com/gauss-project/eswarm/log"
	"github.com/gauss-project/eswarm/rpc"
)

type StdIOUI struct {
	client rpc.Client
	mu     sync.Mutex
}

func NewStdIOUI() *StdIOUI {
	client, err := rpc.DialContext(context.Background(), "stdio://")
	if err != nil {
		log.Crit("Could not create stdio client", "err", err)
	}
	ui := &StdIOUI{client: *client}
	return ui
}

func (ui *StdIOUI) RegisterUIServer(api *UIServerAPI) {
	ui.client.RegisterName("clef", api)
}

// dispatch sends a request over the stdio
func (ui *StdIOUI) dispatch(serviceMethod string, args interface{}, reply interface{}) error {
	err := ui.client.Call(&reply, serviceMethod, args)
	if err != nil {
		log.Info("Error", "exc", err.Error())
	}
	return err
}

// notify sends a request over the stdio, and does not listen for a response
func (ui *StdIOUI) notify(serviceMethod string, args interface{}) error {
	ctx := context.Background()
	err := ui.client.Notify(ctx, serviceMethod, args)
	if err != nil {
		log.Info("Error", "exc", err.Error())
	}
	return err
}

func (ui *StdIOUI) ApproveTx(request *SignTxRequest) (SignTxResponse, error) {
	var result SignTxResponse
	err := ui.dispatch("ApproveTx", request, &result)
	return result, err
}

func (ui *StdIOUI) ApproveSignData(request *SignDataRequest) (SignDataResponse, error) {
	var result SignDataResponse
	err := ui.dispatch("ApproveSignData", request, &result)
	return result, err
}

func (ui *StdIOUI) ApproveExport(request *ExportRequest) (ExportResponse, error) {
	var result ExportResponse
	err := ui.dispatch("ApproveExport", request, &result)
	return result, err
}

func (ui *StdIOUI) ApproveImport(request *ImportRequest) (ImportResponse, error) {
	var result ImportResponse
	err := ui.dispatch("ApproveImport", request, &result)
	return result, err
}

func (ui *StdIOUI) ApproveListing(request *ListRequest) (ListResponse, error) {
	var result ListResponse
	err := ui.dispatch("ApproveListing", request, &result)
	return result, err
}

func (ui *StdIOUI) ApproveNewAccount(request *NewAccountRequest) (NewAccountResponse, error) {
	var result NewAccountResponse
	err := ui.dispatch("ApproveNewAccount", request, &result)
	return result, err
}

func (ui *StdIOUI) ShowError(message string) {
	err := ui.notify("ShowError", &Message{message})
	if err != nil {
		log.Info("Error calling 'ShowError'", "exc", err.Error(), "msg", message)
	}
}

func (ui *StdIOUI) ShowInfo(message string) {
	err := ui.notify("ShowInfo", Message{message})
	if err != nil {
		log.Info("Error calling 'ShowInfo'", "exc", err.Error(), "msg", message)
	}
}
func (ui *StdIOUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	err := ui.notify("OnApprovedTx", tx)
	if err != nil {
		log.Info("Error calling 'OnApprovedTx'", "exc", err.Error(), "tx", tx)
	}
}

func (ui *StdIOUI) OnSignerStartup(info StartupInfo) {
	err := ui.notify("OnSignerStartup", info)
	if err != nil {
		log.Info("Error calling 'OnSignerStartup'", "exc", err.Error(), "info", info)
	}
}
func (ui *StdIOUI) OnInputRequired(info UserInputRequest) (UserInputResponse, error) {
	var result UserInputResponse
	err := ui.dispatch("OnInputRequired", info, &result)
	if err != nil {
		log.Info("Error calling 'OnInputRequired'", "exc", err.Error(), "info", info)
	}
	return result, err
}
