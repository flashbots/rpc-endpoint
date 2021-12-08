package server

import (
	"fmt"
	"strings"

	"github.com/flashbots/rpc-endpoint/types"
)

var ProtectTxApiHost = "https://protect.flashbots.net"

// If public getTransactionReceipt of a submitted tx is null, then check internal API to see if tx has failed
func (r *RpcRequest) check_post_getTransactionReceipt(jsonResp *types.JsonRpcResponse) (requestFinished bool) {
	if jsonResp == nil {
		return false
	}

	resultStr := string(jsonResp.Result)
	if resultStr != "null" {
		return false
	}

	if len(r.jsonReq.Params) < 1 {
		return false
	}

	txHashLower := strings.ToLower(r.jsonReq.Params[0].(string))
	r.log("[post_getTransactionReceipt] eth_getTransactionReceipt is null, check if it was a private tx: %s", txHashLower)

	// get tx status from private-tx-api
	statusApiResponse, err := GetTxStatus(txHashLower)
	if err != nil {
		r.logError("[post_getTransactionReceipt] privateTxApi failed: %s", err)
		return false
	}

	ensureAccountFixIsInPlace := func() {
		// Get the sender of this transaction
		txFromLower, txFromFound, err := RState.GetSenderOfTxHash(txHashLower)
		if err != nil {
			r.logError("[post_getTransactionReceipt] redis:GetSenderOfTxHash failed: %v", err)
			return
		}

		if !txFromFound { // cannot sent nonce-fix if we don't have the sender
			return
		}

		// Check if nonceFix is already in place for this user
		_, nonceFixAlreadyExists, err := RState.GetNonceFixForAccount(txFromLower)
		if err != nil {
			r.logError("[post_getTransactionReceipt] redis:GetNonceFixForAccount failed: %s", err)
			return
		}

		if nonceFixAlreadyExists {
			return
		}

		// Setup a new nonce-fix for this user
		err = RState.SetNonceFixForAccount(txFromLower, 0)
		if err != nil {
			r.logError("[post_getTransactionReceipt] redis error: %s", err)
			return
		}

		r.log("[post_getTransactionReceipt] nonce-fix set for: %s", txFromLower)
	}

	r.log("[post_getTransactionReceipt] priv-tx-api status: %s", statusApiResponse.Status)
	if statusApiResponse.Status == types.TxStatusFailed || (DebugDontSendTx && statusApiResponse.Status == types.TxStatusUnknown) {
		r.log("[post_getTransactionReceipt] failed private tx, ensure account fix is in place")
		ensureAccountFixIsInPlace()
		// r.writeRpcError("Transaction failed") // If this is sent before metamask dropped the tx (received 4x invalid nonce), then it doesn't call getTransactionCount anymore
		// TODO: return standard failed tx payload?
		return false

		// } else if statusApiResponse.Status == types.TxStatusIncluded {
		// 	// NOTE: This branch can never happen, because if tx is included then Receipt will not return null
		// 	// TODO? If latest tx of this user was a successful, then we should remove the nonce fix
		// 	// This could lead to a ping-pong between checking 2 tx, with one check adding and another removing the nonce fix
		// 	// See also the branch tmp-check_post_getTransactionReceipt-removeNonceFix
		// 	_ = 1
	}

	return false
}

func (r *RpcRequest) intercept_mm_eth_getTransactionCount() (requestFinished bool) {
	if len(r.jsonReq.Params) < 1 {
		return false
	}

	addr := strings.ToLower(r.jsonReq.Params[0].(string))

	// Check if nonceFix is in place for this user
	numTimesSent, nonceFixInPlace, err := RState.GetNonceFixForAccount(addr)
	if err != nil {
		r.logError("redis:GetAccountWithNonceFix error:", err)
		return false
	}

	if !nonceFixInPlace {
		return false
	}

	// Intercept max 4 times (after which Metamask marks it as dropped)
	numTimesSent += 1
	if numTimesSent > 4 {
		return false
	}

	err = RState.SetNonceFixForAccount(addr, numTimesSent)
	if err != nil {
		r.logError("redis:SetAccountWithNonceFix error:", err)
		return false
	}

	r.log("eth_getTransactionCount intercept: #%d", numTimesSent)

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
