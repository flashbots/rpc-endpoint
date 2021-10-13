package server

import (
	"fmt"
	"strings"
)

// Returns true if request has already received a response, false if req should contiue to normal proxy
func (r *RpcRequest) intercept_mm_eth_getTransactionCount() (requestFinished bool) {
	if len(r.jsonReq.Params) < 1 {
		return false
	}

	addr := strings.ToLower(r.jsonReq.Params[0].(string))
	mmHelperBlacklistEntry, mmHelperBlacklistEntryFound := mmBlacklistedAccountAndNonce[addr]
	if !mmHelperBlacklistEntryFound {
		return false
	}

	// MM should get nonce+1 four times to stop resending
	mmBlacklistedAccountAndNonce[addr].NumTries += 1
	if mmBlacklistedAccountAndNonce[addr].NumTries == 4 {
		delete(mmBlacklistedAccountAndNonce, addr)
	}

	// Prepare custom JSON-RPC response
	r.writeRpcResult(fmt.Sprintf("0x%x", mmHelperBlacklistEntry.Nonce+1))
	r.log("Intercepted eth_getTransactionCount for %s", addr)
	return true
}

// Returns true if request has already received a response, false if req should contiue to normal proxy
func (r *RpcRequest) intercept_eth_call_to_FlashRPC_Contract() (requestFinished bool) {
	if len(r.jsonReq.Params) < 1 {
		return false
	}

	ethCallReq := r.jsonReq.Params[0].(map[string]interface{})
	addressTo := strings.ToLower(ethCallReq["to"].(string))

	// Only handle calls to the Flashbots RPC check contract
	// 0xf1a54b075 --> 0xflashbots
	// https://etherscan.io/address/0xf1a54b0759b58661cea17cff19dd37940a9b5f1a#readContract
	if addressTo != "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a" {
		return false
	}

	r.writeRpcResult("0x0000000000000000000000000000000000000000000000000000000000000001")
	r.log("Intercepted eth_call")
	return true
}
