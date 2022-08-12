package server

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flashbots/rpc-endpoint/types"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

const (
	scMethodBytes = 4 // first 4 byte of data field
)

func (r *RpcRequest) handle_sendRawTransaction() {
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
	// Get address from tx
	r.txFrom, err = GetSenderFromRawTx(r.tx)
	if err != nil {

		r.logger.Info("[sendRawTransaction] Couldn't get address from rawTx", "error", err)
		r.writeRpcError(fmt.Sprintf("couldn't get address from rawTx: %v", err), types.JsonRpcInvalidRequest)
		return
	}

	r.logger.Info("[sendRawTransaction] sending raw transaction", "tx", r.rawTxHex, "txHash", r.tx.Hash(), "fromAddress", r.txFrom, "toAddress", AddressPtrToStr(r.tx.To()), "txNonce", r.tx.Nonce(), "txGasPrice", BigIntPtrToStr(r.tx.GasPrice()), "ip", r.ip)
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
		r.logger.Info("[sendRawTransaction] tx rejected - nonce too high", "txNonce", r.tx.Nonce(), "txHash", r.tx.Hash(), "txFromLower", txFromLower, "origin", r.origin)
		r.writeRpcError("tx rejected - nonce too high", types.JsonRpcInvalidRequest)
		return
	}

	txHashLower := strings.ToLower(r.tx.Hash().Hex())
	// Check if tx was blocked (eg. "nonce too low")
	retVal, isBlocked, _ := RState.GetBlockedTxHash(txHashLower)
	if isBlocked {
		r.logger.Info("[sendRawTransaction] tx blocked", "txHash", r.tx.Hash(), "retVal", retVal)
		r.writeRpcError(retVal, types.JsonRpcInternalError)
		return
	}

	// Remember sender of the tx, for lookup in getTransactionReceipt to possibly set nonce-fix
	err = RState.SetSenderOfTxHash(txHashLower, txFromLower)
	if err != nil {
		r.logger.Error("[sendRawTransaction] Redis:SetSenderOfTxHash failed: %v", err)
	}

	// Check if transaction needs protection
	needsProtection := r.doesTxNeedFrontrunningProtection(r.tx)
	r.ethSendRawTxEntry.NeedsFrontRunningProtection = needsProtection
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
		requestDone := r.handleCancelTx()       // returns true if tx was cancelled at the relay and response has been sent to the user
		if requestDone && !r.preferences.Fast { // a cancel-tx to fast endpoint is also sent to mempool
			return
		}

		// It's a cancel-tx for the mempool
		needsProtection = false
		r.logger.Info("[cancel-tx] Sending to mempool", "txFromLower", txFromLower, "txNonce", r.tx.Nonce())
	}

	if needsProtection {
		r.sendTxToRelay()
		return
	}

	if DebugDontSendTx {
		r.logger.Info("[sendRawTransaction] Faked sending tx to mempool, did nothing")
		r.writeRpcResult(r.tx.Hash().Hex())
		return
	}

	// Proxy to public node now
	readJsonRpcSuccess := r.proxyRequestRead()
	r.ethSendRawTxEntry.WasSentToMempool = true
	// Log after proxying
	if !readJsonRpcSuccess {
		r.logger.Error("[sendRawTransaction] Proxy to mempool failed")
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return
	}

	// at the end, save the nonce for further spam protection checks
	go RState.SetSenderMaxNonce(txFromLower, r.tx.Nonce())

	if r.jsonRes.Error != nil {
		r.logger.Info("[sendRawTransaction] Proxied eth_sendRawTransaction to mempool", "jsonRpcError", r.jsonRes.Error.Message, "txHash", r.tx.Hash())
		r.ethSendRawTxEntry.Error = r.jsonRes.Error.Message
		r.ethSendRawTxEntry.ErrorCode = r.jsonRes.Error.Code
		if r.jsonRes.Error.Message == "nonce too low" {
			RState.SetBlockedTxHash(txHashLower, "nonce too low")
		}
	} else {
		r.logger.Info("[sendRawTransaction] Proxied eth_sendRawTransaction to mempool", "txHash", r.tx.Hash())
	}
}

// Check if a request needs frontrunning protection. There are many transactions that don't need frontrunning protection,
// for example simple ERC20 transfers.
func (r *RpcRequest) doesTxNeedFrontrunningProtection(tx *ethtypes.Transaction) bool {
	gas := tx.Gas()
	r.logger.Info("[protect-check]", "gas", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false
	}

	data := hex.EncodeToString(tx.Data())
	r.logger.Info("[protect-check] ", "tx-data", data)

	if len(data) < 8 {
		return false
	}

	if isOnFunctionWhitelist(data[0:8]) {
		return false // function being called is on our whitelist and no protection needed
	} else {
		r.logger.Info("[protect-check] Tx needs protection - function", "tx-data", data[0:8])
		return true // needs protection if not on whitelist
	}
}
