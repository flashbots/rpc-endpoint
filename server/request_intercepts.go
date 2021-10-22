package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// After getTransactionReceipt, check if result is null, and if >16 min since submission, query TxManager BE (MM fix 2)
func (r *RpcRequest) check_post_getTransactionReceipt(jsonResp *JsonRpcResponse) {
	resultStr := string(jsonResp.Result)
	if resultStr != "null" {
		return
	}

	if len(r.jsonReq.Params) < 1 {
		return
	}

	txHash := r.jsonReq.Params[0].(string)
	r.log("[MM2] eth_getTransactionReceipt for tx %s", txHash)

	rawTxSubmission, txFound := MetaMaskFix.rawTransactionSubmission[strings.ToLower(txHash)]
	if !txFound {
		return
	}

	td := time.Since(rawTxSubmission.submittedAt)
	if td < 17*time.Minute { // do nothing until at least 16 minutes passed
		return
	}

	r.log("[MM2] eth_getTransactionReceipt result came back empty and > 16 min, tx %s", txHash)

	// result null, and sent before, but more than 16 min ago. Call eth_getBundleStatusByTransactionHash on BE now
	req := NewJsonRpcRequest1(1, "eth_getBundleStatusByTransactionHash", txHash)
	backendResp, err := SendRpcAndParseResponseTo(r.txManagerUrl, req)
	if err != nil {
		r.logError("[MM2] eth_getBundleStatusByTransactionHash failed for %s: %s", txHash, err)
		return
	}
	if backendResp.Error != nil {
		r.logError("[MM2] eth_getBundleStatusByTransactionHash failed for %s (BE error): %s", txHash, backendResp.Error.Message)
		return
	}
	r.log("[MM2] BE response: %s", string(backendResp.Result))

	statusResponse := new(GetBundleStatusByTransactionHashResponse)
	err = json.Unmarshal(backendResp.Result, &statusResponse)
	if err != nil {
		r.logError("[MM2] eth_getBundleStatusByTransactionHash failed unmarshal rpc result for %s: %s - %s", txHash, jsonResp.Result, err)
		return
	}

	// r.log("[MM2] BE response: %d, %v", statusResponse)

	if statusResponse.Status == "FAILED_BUNDLE" {
		r.log("[MM2] blacklisted tx hash, will receive too high of a nonce")
		mmRawTxTrack, found := MetaMaskFix.rawTransactionSubmission[strings.ToLower(txHash)]
		if !found {
			r.logError("[MM2] couldn't find previous transaction")
			return
		}

		MetaMaskFix.blacklistedRawTx[strings.ToLower(txHash)] = Now()
		MetaMaskFix.accountAndNonce[strings.ToLower(mmRawTxTrack.txFrom)] = &mmNonceHelper{
			Nonce: 1e9,
		}
	}
}

func (r *RpcRequest) intercept_mm_eth_getTransactionCount() (requestFinished bool) {
	if len(r.jsonReq.Params) < 1 {
		return false
	}

	addr := strings.ToLower(r.jsonReq.Params[0].(string))
	mmHelperBlacklistEntry, mmHelperBlacklistEntryFound := MetaMaskFix.accountAndNonce[addr]
	if !mmHelperBlacklistEntryFound {
		return false
	}

	// MM should get nonce+1 four times to stop resending
	MetaMaskFix.accountAndNonce[addr].NumTries += 1
	if MetaMaskFix.accountAndNonce[addr].NumTries == 4 {
		delete(MetaMaskFix.accountAndNonce, addr)
	}

	// Prepare custom JSON-RPC response
	resp := fmt.Sprintf("0x%x", mmHelperBlacklistEntry.Nonce+1)
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
