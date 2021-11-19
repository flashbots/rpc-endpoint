package server

import (
	"fmt"
	"strings"
)

var ProtectTxApiHost = "https://protect.flashbots.net"

// If public getTransactionReceipt of a submitted tx is null, then check internal API to see if tx has failed
func (r *RpcRequest) check_post_getTransactionReceipt(jsonResp *JsonRpcResponse) (requestFinished bool) {
	resultStr := string(jsonResp.Result)
	if resultStr != "null" {
		return
	}

	if len(r.jsonReq.Params) < 1 {
		return
	}

	txHashLower := strings.ToLower(r.jsonReq.Params[0].(string))

	// Abort if transaction wasn't submitted before
	txFrom, txFound := State.txHashToUser[txHashLower]
	if !txFound {
		return
	}
	txFromLower := txFrom.s

	// Abort if not the latest transaction by user
	latestTxHash, latestFound := State.userLatestTxHash[txFromLower]
	if latestFound && latestTxHash.s != txHashLower {
		return
	}

	// Remove any nonce fix for an earlier tx
	nonceFix, nonceFixAlreadyInPlace := State.accountWithNonceFix[txFromLower]
	if nonceFixAlreadyInPlace {
		if nonceFix.txHash != txHashLower {
			// nonce fix for a different tx: delete state and proceed
			delete(State.accountWithNonceFix, txFromLower)
		} else {
			// nonce fix for the latest tx. do nothing until user has received 4x the fixed nonce, then reject with error
			if nonceFix.numTries >= 4 {
				r.writeRpcError("private tx failed")
				return true
			}
			return false
		}
	}

	r.log("[MM2] eth_getTransactionReceipt is null for latest user tx %s", txHashLower)

	// get tx status from private-tx-api
	statusApiResponse, err := GetTxStatus(txHashLower)
	if err != nil {
		r.logError("[MM2] privateTxApi failed: %s", err)
		return
	}

	r.log("[MM2] priv-tx-api status: %s", statusApiResponse.Status)
	if statusApiResponse.Status == "FAILED" || (DebugDontSendTx && statusApiResponse.Status == "UNKNOWN") {
		r.log("[MM2] failed tx, will receive too high of a nonce")
		State.accountWithNonceFix[txFromLower] = NewNonceFix(txHashLower)
	} else {
		// healthy response, remove any nonce fix
		delete(State.accountWithNonceFix, txFromLower)
	}

	return false
}

func (r *RpcRequest) intercept_mm_eth_getTransactionCount() (requestFinished bool) {
	if len(r.jsonReq.Params) < 1 {
		return false
	}

	addr := strings.ToLower(r.jsonReq.Params[0].(string))

	// Check if nonceFix is in place for this user
	nonceFix, shouldInterceptNonce := State.accountWithNonceFix[addr]
	if !shouldInterceptNonce {
		return false
	}

	// Intercept max 4 times (after which Metamask marks it as dropped)
	nonceFix.numTries += 1
	if nonceFix.numTries > 4 {
		return
	}

	r.log("eth_getTransactionCount intercept: #%d", nonceFix.numTries)

	// Return invalid nonce
	var wrongNonce uint64 = 1e9 + 1
	resp := fmt.Sprintf("0x%x", wrongNonce)
	r.writeRpcResult(resp)
	r.log("Intercepted eth_getTransactionCount for %s", addr)
	return true
}

// Returns true if request has already received a response, false if req should contiue to normal proxy
func (r *RpcRequest) intercept_eth_call_to_FlashRPC_Contract() (requestFinished bool) {
	if len(r.jsonReq.Params) < 1 {
		return false
	}

	ethCallReq := r.jsonReq.Params[0].(map[string]interface{})
	if ethCallReq["to"] == nil {
		return false
	}

	addressTo := strings.ToLower(ethCallReq["to"].(string))

	// Only handle calls to the Flashbots RPC check contract
	// 0xf1a54b075 --> 0xflashbots
	// https://etherscan.io/address/0xf1a54b0759b58661cea17cff19dd37940a9b5f1a#readContract
	if addressTo != "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a" {
		return false
	}

	r.writeRpcResult("0x0000000000000000000000000000000000000000000000000000000000000001")
	r.log("Intercepted eth_call to FlashRPC contract")
	return true
}
