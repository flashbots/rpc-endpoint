package server

import (
	"encoding/hex"
	"net/http"
	"strings"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/rpc-endpoint/utils"
)

func (r *RpcRequest) handle_sendRawTransaction() {
	var err error

	// JSON-RPC sanity checks
	if len(r.jsonReq.Params) < 1 {
		r.logError("no params for eth_sendRawTransaction")
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.rawTxHex = r.jsonReq.Params[0].(string)
	if len(r.rawTxHex) < 2 {
		r.logError("invalid raw transaction (wrong length)")
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.log("rawTx: %s", r.rawTxHex)

	r.tx, err = GetTx(r.rawTxHex)
	if err != nil {
		r.log("reading transaction object failed - rawTx: %s", r.rawTxHex)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	// Get tx from address
	r.txFrom, err = GetSenderFromRawTx(r.tx)
	if err != nil {
		r.log("couldn't get address from rawTx: %v", err)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.log("txHash: %s - from: %s / to: %s / nonce: %d / gasPrice: %s", r.tx.Hash(), r.txFrom, utils.AddressPtrToStr(r.tx.To()), r.tx.Nonce(), utils.BigIntPtrToStr(r.tx.GasPrice()))
	txFromLower := strings.ToLower(r.txFrom)

	if r.tx.Nonce() >= 1e9 {
		r.log("tx rejected - nonce too high: %d - %s from %s / origin: %s", r.tx.Nonce(), r.tx.Hash(), txFromLower, r.origin)
		r.writeRpcError("tx rejected - nonce too high")
		return
	}

	txHashLower := strings.ToLower(r.tx.Hash().Hex())

	// Remember sender of the tx, for lookup in getTransactionReceipt to possibly set nonce-fix
	err = RState.SetSenderOfTxHash(txHashLower, txFromLower)
	if err != nil {
		r.logError("redis:SetSenderOfTxHash failed: %v", err)
	}

	if isOnOFACList(r.txFrom) {
		r.log("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
		r.writeHeaderStatus(http.StatusUnauthorized)
		return
	}

	// Check if transaction needs protection
	needsProtection := r.doesTxNeedFrontrunningProtection(r.tx)

	// Check for cancellation-tx
	if len(r.tx.Data()) <= 2 && txFromLower == strings.ToLower(r.tx.To().Hex()) {
		requestDone := r.handleCancelTx() // returns true if tx was cancelled at the relay and response has been sent to the user
		if requestDone {
			return
		}

		// It's a cancel-tx for the mempool
		needsProtection = false
		r.log("[cancel-tx] sending to mempool for %s/%d", txFromLower, r.tx.Nonce())
	}

	if needsProtection {
		r.sendTxToRelay()
		return
	}

	if DebugDontSendTx {
		r.log("faked sending tx to mempool, did nothing")
		r.writeRpcResult(r.tx.Hash().Hex())
		return
	}

	// Proxy to public node now
	readJsonRpcSuccess, proxyHttpStatus, jsonResp := r.proxyRequestRead(r.defaultProxyUrl)

	// Log after proxying
	if !readJsonRpcSuccess {
		r.logError("Proxy to mempool failed: eth_sendRawTransaction")
		r.writeHeaderStatus(http.StatusInternalServerError)
		return
	}

	// Write JSON-RPC response now
	r.writeHeaderContentTypeJson()
	r.writeHeaderStatus(proxyHttpStatus)
	r._writeRpcResponse(jsonResp)

	if jsonResp.Error != nil {
		r.log("Proxied eth_sendRawTransaction to mempool - with JSON-RPC Error %s", jsonResp.Error.Message)
	} else {
		r.log("Proxied eth_sendRawTransaction to mempool")
	}
}

// Check if a request needs frontrunning protection. There are many transactions that don't need frontrunning protection,
// for example simple ERC20 transfers.
func (r *RpcRequest) doesTxNeedFrontrunningProtection(tx *ethtypes.Transaction) bool {
	gas := tx.Gas()
	r.log("[protect-check] gas: %v", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false
	}

	data := hex.EncodeToString(tx.Data())
	r.log("[protect-check] tx-data: %v", data)

	if len(data) < 8 {
		return false
	}

	if isOnFunctionWhiteList(data[0:8]) {
		return false // function being called is on our whitelist and no protection needed
	} else {
		r.log("[protect-check] tx needs protection - function: %v", data[0:8])
		return true // needs protection if not on whitelist
	}
}
