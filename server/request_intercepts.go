package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Check if getTransactionReceipt of a submitted tx is null. If submitted longer than time threshold ago, query TxManager BE to see if tx failed.
func (r *RpcRequest) check_post_getTransactionReceipt(jsonResp *JsonRpcResponse) {
	resultStr := string(jsonResp.Result)
	if resultStr != "null" {
		return
	}

	if len(r.jsonReq.Params) < 1 {
		return
	}

	txHash := r.jsonReq.Params[0].(string)
	rawTxSubmission, txFound := MetaMaskFix.rawTransactionSubmission[strings.ToLower(txHash)]
	if !txFound {
		return
	}

	td := time.Since(rawTxSubmission.submittedAt)
	minutesSinceSubmission := td.Minutes()
	r.log("[MM2] check_post_getTransactionReceipt for tx %s - submittedAt %.2f min ago", txHash, minutesSinceSubmission)

	maxTime := 14
	if r.useRelay {
		maxTime = 1 // 25 blocks max
	}

	if minutesSinceSubmission < float64(maxTime) { // do nothing until at least 14 minutes passed
		return
	}

	r.log("[MM2] eth_getTransactionReceipt result came back empty and > time threshold: tx %s", txHash)

	setMmNonceFix := func(txHash string) {
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

	if r.useRelay {
		// call private-tx-api
		privTxApiUrl := fmt.Sprintf("http://3.21.58.202/tx/%s", txHash)
		resp, err := http.Get(privTxApiUrl)
		if err != nil {
			r.logError("[MM2] privTxApi call failed for %s (BE error): %s", txHash, err)
			return
		}
		defer resp.Body.Close()

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			r.logError("[MM2] privTxApi call body-read failed for %s (BE error): %s", txHash, err)
			return
		}

		r.log("[MM2] BE response: %s", string(bodyBytes))
		respObj := new(PrivateTxApiResponse)
		err = json.Unmarshal(bodyBytes, respObj)
		if err != nil {
			r.logError("[MM2] privTxApi call json-unmarshal failed for %s (BE error): %s", txHash, err)
			return
		}

		if respObj.Status == "FAILED" {
			setMmNonceFix(txHash)
		}

	} else {
		// call TxManager:eth_getBundleStatusByTransactionHash
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
			setMmNonceFix(txHash)
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
