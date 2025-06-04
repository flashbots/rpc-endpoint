package server

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flashbots/rpc-endpoint/metrics"
	"github.com/flashbots/rpc-endpoint/types"
)

const (
	scMethodBytes = 4 // first 4 byte of data field
)

func (r *RpcRequest) handle_sendRawTransaction() {
	metrics.IncPrivateTx()

	var err error

	// JSON-RPC sanity checks
	if len(r.jsonReq.Params) < 1 {
		r.logger.Info("[sendRawTransaction] No params for eth_sendRawTransaction")
		r.writeRpcError("empty params for eth_sendRawTransaction", types.JsonRpcInvalidParams)
		return
	}

	if r.jsonReq.Params[0] == nil {
		r.logger.Info("[sendRawTransaction]  Nil param for eth_sendRawTransaction")
		r.writeRpcError("nil params for eth_sendRawTransaction", types.JsonRpcInvalidParams)
	}

	r.rawTxHex = r.jsonReq.Params[0].(string)
	if len(r.rawTxHex) < 2 {
		r.logger.Error("[sendRawTransaction] Invalid raw transaction (wrong length)")
		r.writeRpcError("invalid raw transaction param (wrong length)", types.JsonRpcInvalidParams)
		return
	}

	// r.logger.Info("[sendRawTransaction] Raw tx value", "tx", r.rawTxHex, "txHash", r.tx.Hash())
	r.ethSendRawTxEntry.TxRaw = r.rawTxHex
	r.tx, err = GetTx(r.rawTxHex)
	if err != nil {
		r.logger.Info("[sendRawTransaction] Reading transaction object failed", "tx", r.rawTxHex)
		r.writeRpcError(fmt.Sprintf("reading transaction object failed - rawTx: %s", r.rawTxHex), types.JsonRpcInvalidRequest)
		return
	}
	r.ethSendRawTxEntry.TxHash = r.tx.Hash().String()
	r.logger = r.logger.New("txHash", r.tx.Hash().String())
	// Get address from tx
	r.logger.Info("[sendRawTransaction] start to process raw tx", "txHash", r.tx.Hash(), "timestamp", time.Now().Unix(), "time", time.Now().UTC())
	r.txFrom, err = GetSenderFromRawTx(r.tx)
	if err != nil {

		r.logger.Info("[sendRawTransaction] Couldn't get address from rawTx", "error", err)
		r.writeRpcError(fmt.Sprintf("couldn't get address from rawTx: %v", err), types.JsonRpcInvalidRequest)
		return
	}

	r.logger.Info("[sendRawTransaction] sending raw transaction", "tx", r.rawTxHex, "fromAddress", r.txFrom, "toAddress", AddressPtrToStr(r.tx.To()), "txNonce", r.tx.Nonce(), "txGasPrice", BigIntPtrToStr(r.tx.GasPrice()))
	txFromLower := strings.ToLower(r.txFrom)

	// store tx info to ethSendRawTxEntries which will be stored in db for data analytics reason
	r.ethSendRawTxEntry.TxFrom = r.txFrom
	r.ethSendRawTxEntry.TxTo = AddressPtrToStr(r.tx.To())
	r.ethSendRawTxEntry.TxNonce = int(r.tx.Nonce())

	if len(r.tx.Data()) > 0 {
		r.ethSendRawTxEntry.TxData = hexutil.Encode(r.tx.Data())
	}

	if len(r.tx.Data()) >= scMethodBytes {
		r.ethSendRawTxEntry.TxSmartContractMethod = hexutil.Encode(r.tx.Data()[:scMethodBytes])
	}

	if r.tx.Nonce() >= 1e9 {
		r.logger.Info("[sendRawTransaction] tx rejected - nonce too high", "txNonce", r.tx.Nonce(), "txFromLower", txFromLower, "origin", r.origin)
		r.writeRpcError("tx rejected - nonce too high", types.JsonRpcInvalidRequest)
		return
	}

	txHashLower := strings.ToLower(r.tx.Hash().Hex())
	// Check if tx was blocked (eg. "nonce too low")
	retVal, isBlocked, _ := RState.GetBlockedTxHash(txHashLower)
	if isBlocked {
		r.logger.Info("[sendRawTransaction] tx blocked", "retVal", retVal)
		r.writeRpcError(retVal, types.JsonRpcInternalError)
		return
	}

	// Remember sender and nonce of the tx, for lookup in getTransactionReceipt to possibly set nonce-fix
	err = RState.SetSenderAndNonceOfTxHash(txHashLower, txFromLower, r.tx.Nonce())
	if err != nil {
		r.logger.Error("[sendRawTransaction] Redis:SetSenderAndNonceOfTxHash failed: %v", err)
	}
	var txToAddr string
	if r.tx.To() != nil { // to address will be nil for contract creation tx
		txToAddr = r.tx.To().String()
	}
	isOnOfacList := isOnOFACList(r.txFrom) || isOnOFACList(txToAddr)
	r.ethSendRawTxEntry.IsOnOafcList = isOnOfacList
	if isOnOfacList {
		r.logger.Info("[sendRawTransaction] Blocked tx due to ofac sanctioned address", "txFrom", r.txFrom, "txTo", txToAddr)
		r.writeRpcError("blocked tx due to ofac sanctioned address", types.JsonRpcInvalidRequest)
		return
	}

	// Check if transaction needs protection
	r.ethSendRawTxEntry.NeedsFrontRunningProtection = true
	// If users specify a bundle ID, cache this transaction
	if r.isWhitehatBundleCollection {
		r.logger.Info("[WhitehatBundleCollection] Adding tx to bundle", "whiteHatBundleId", r.whitehatBundleId, "tx", r.rawTxHex)
		err = RState.AddTxToWhitehatBundle(r.whitehatBundleId, r.rawTxHex)
		if err != nil {
			r.logger.Error("[WhitehatBundleCollection] AddTxToWhitehatBundle failed", "error", err)
			r.writeRpcError("[WhitehatBundleCollection] AddTxToWhitehatBundle failed:", types.JsonRpcInternalError)
			return
		}
		r.writeRpcResult(r.tx.Hash().Hex())
		return
	}

	// Check for cancellation-tx
	if r.tx.To() != nil && len(r.tx.Data()) <= 2 && txFromLower == strings.ToLower(r.tx.To().Hex()) {
		r.ethSendRawTxEntry.IsCancelTx = true
		requestDone := r.handleCancelTx() // returns true if tx was cancelled at the relay and response has been sent to the user
		if !requestDone {
			r.ethSendRawTxEntry.IsCancelTx = false
			r.logger.Warn("[cancel-tx] This is not a cancellation tx, since we don't have original one. So we process it as usual tx", "txFromLower", txFromLower, "txNonce", r.tx.Nonce())
			r.sendTxToRelay()
		}
		return
	}

	// do it as the last step, in case it is used as cancellation
	if r.tx.GasTipCap().Cmp(big.NewInt(0)) == 0 {
		r.writeRpcError("transaction underpriced: gas tip cap 0, minimum needed 1", types.JsonRpcInvalidRequest)
		return
	}
	r.sendTxToRelay()
}
