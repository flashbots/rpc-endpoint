package server

import (
	"encoding/hex"
	"fmt"
	"github.com/flashbots/rpc-endpoint/types"
	"strings"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/rpc-endpoint/utils"
)

func (r *RpcRequest) handle_sendRawTransaction() {
	var err error

	// JSON-RPC sanity checks
	if len(r.jsonReq.Params) < 1 {
		r.logger.log("no params for eth_sendRawTransaction")
		r.writeRpcError("empty params for eth_sendRawTransaction", types.JsonRpcInvalidParams)
		return
	}

	if r.jsonReq.Params[0] == nil {
		r.logger.log("nil param for eth_sendRawTransaction")
		r.writeRpcError("nil params for eth_sendRawTransaction", types.JsonRpcInvalidParams)
	}

	r.rawTxHex = r.jsonReq.Params[0].(string)
	if len(r.rawTxHex) < 2 {
		r.logger.logError("invalid raw transaction (wrong length)")
		r.writeRpcError("invalid raw transaction param (wrong length)", types.JsonRpcInvalidParams)
		return
	}

	r.logger.log("rawTx: %s", r.rawTxHex)

	r.tx, err = GetTx(r.rawTxHex)
	if err != nil {
		r.logger.log("reading transaction object failed - rawTx: %s", r.rawTxHex)
		r.writeRpcError(fmt.Sprintf("reading transaction object failed - rawTx: %s", r.rawTxHex), types.JsonRpcInvalidRequest)
		return
	}

	// Get tx from address
	r.txFrom, err = GetSenderFromRawTx(r.tx)
	if err != nil {
		r.logger.log("couldn't get address from rawTx: %v", err)
		r.writeRpcError(fmt.Sprintf("couldn't get address from rawTx: %v", err), types.JsonRpcInvalidRequest)
		return
	}

	r.logger.log("txHash: %s - from: %s / to: %s / nonce: %d / gasPrice: %s", r.tx.Hash(), r.txFrom, utils.AddressPtrToStr(r.tx.To()), r.tx.Nonce(), utils.BigIntPtrToStr(r.tx.GasPrice()))
	txFromLower := strings.ToLower(r.txFrom)

	if r.tx.Nonce() >= 1e9 {
		r.logger.log("tx rejected - nonce too high: %d - %s from %s / origin: %s", r.tx.Nonce(), r.tx.Hash(), txFromLower, r.origin)
		r.writeRpcError("tx rejected - nonce too high", types.JsonRpcInvalidRequest)
		return
	}

	txHashLower := strings.ToLower(r.tx.Hash().Hex())

	// Remember sender of the tx, for lookup in getTransactionReceipt to possibly set nonce-fix
	err = RState.SetSenderOfTxHash(txHashLower, txFromLower)
	if err != nil {
		r.logger.logError("redis:SetSenderOfTxHash failed: %v", err)
	}

	if isOnOFACList(r.txFrom) {
		r.logger.log("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
		r.writeRpcError("blocked tx from ofac sanctioned address", types.JsonRpcInvalidRequest)
		return
	}

	// Check if transaction needs protection
	needsProtection := r.doesTxNeedFrontrunningProtection(r.tx)

	// If users specify a bundle ID, cache this transaction
	if r.isWhitehatBundleCollection {
		r.logger.log("[WhitehatBundleCollection] adding tx to bundle %s txData: %s", r.whitehatBundleId, r.rawTxHex)
		err = RState.AddTxToWhitehatBundle(r.whitehatBundleId, r.rawTxHex)
		if err != nil {
			r.logger.logError("[WhitehatBundleCollection] AddTxToWhitehatBundle failed:", err)
			r.writeRpcError("[WhitehatBundleCollection] AddTxToWhitehatBundle failed:", types.JsonRpcInternalError)
			return
		}
		r.writeRpcResult(r.tx.Hash().Hex())
		return
	}

	// Check for cancellation-tx
	if len(r.tx.Data()) <= 2 && txFromLower == strings.ToLower(r.tx.To().Hex()) {
		requestDone := r.handleCancelTx() // returns true if tx was cancelled at the relay and response has been sent to the user
		if requestDone {
			return
		}

		// It's a cancel-tx for the mempool
		needsProtection = false
		r.logger.log("[cancel-tx] sending to mempool for %s/%d", txFromLower, r.tx.Nonce())
	}

	if needsProtection {
		r.sendTxToRelay()
		return
	}

	if DebugDontSendTx {
		r.logger.log("faked sending tx to mempool, did nothing")
		r.writeRpcResult(r.tx.Hash().Hex())
		return
	}

	// Proxy to public node now
	readJsonRpcSuccess := r.proxyRequestRead(r.defaultProxyUrl)

	// Log after proxying
	if !readJsonRpcSuccess {
		r.logger.logError("Proxy to mempool failed: eth_sendRawTransaction")
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return
	}

	// at the end, save the nonce for further spam protection checks
	go RState.SetSenderMaxNonce(txFromLower, r.tx.Nonce())

	if r.jsonRes.Error != nil {
		r.logger.log("Proxied eth_sendRawTransaction to mempool - with JSON-RPC Error %s", r.jsonRes.Error.Message)
	} else {
		r.logger.log("Proxied eth_sendRawTransaction to mempool")
	}
}

// Check if a request needs frontrunning protection. There are many transactions that don't need frontrunning protection,
// for example simple ERC20 transfers.
func (r *RpcRequest) doesTxNeedFrontrunningProtection(tx *ethtypes.Transaction) bool {
	gas := tx.Gas()
	r.logger.log("[protect-check] gas: %v", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false
	}

	data := hex.EncodeToString(tx.Data())
	r.logger.log("[protect-check] tx-data: %v", data)

	if len(data) < 8 {
		return false
	}

	if isOnFunctionWhiteList(data[0:8]) {
		return false // function being called is on our whitelist and no protection needed
	} else {
		r.logger.log("[protect-check] tx needs protection - function: %v", data[0:8])
		return true // needs protection if not on whitelist
	}
}
