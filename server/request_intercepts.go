package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

var ProtectTxApiHost = "https://protect.flashbots.net"

// If public getTransactionReceipt of a submitted tx is null, then check internal API to see if tx has failed
func (r *RpcRequest) check_post_getTransactionReceipt(jsonResp *JsonRpcResponse) {
	resultStr := string(jsonResp.Result)
	if resultStr != "null" {
		return
	}

	if len(r.jsonReq.Params) < 1 {
		return
	}

	txHashLower := strings.ToLower(r.jsonReq.Params[0].(string))

	// Make sure transaction was submitted before
	txFrom, txFound := State.txToUser[txHashLower]
	if !txFound {
		return
	}

	// Only check state on latest transaction by user
	txFromLower := txFrom.s
	latestTxHash, latestFound := State.userLatestTx[txFromLower]
	if latestFound && latestTxHash.s != txHashLower {
		return
	}

	r.log("[MM2] eth_getTransactionReceipt is null for latest user tx %s", txHashLower)

	// call private-tx-api
	privTxApiUrl := fmt.Sprintf("%s/tx/%s", ProtectTxApiHost, txHashLower)
	resp, err := http.Get(privTxApiUrl)
	if err != nil {
		r.logError("[MM2] privTxApi call failed for %s (BE error): %s", txHashLower, err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		r.logError("[MM2] privTxApi call body-read failed for %s (BE error): %s", txHashLower, err)
		return
	}

	respObj := new(PrivateTxApiResponse)
	err = json.Unmarshal(bodyBytes, respObj)
	if err != nil {
		r.log("[MM2] priv-tx-api response: %s", string(bodyBytes))
		r.logError("[MM2] privTxApi call json-unmarshal failed for %s (BE error): %s", txHashLower, err)
		return
	}

	r.log("[MM2] priv-tx-api status: %s", respObj.Status)
	if respObj.Status == "FAILED" {
		r.log("[MM2] blacklisted tx hash, will receive too high of a nonce")
		State.accountWithNonceFix[txFromLower] = Now()
	}
}

func (r *RpcRequest) intercept_mm_eth_getTransactionCount() (requestFinished bool) {
	if len(r.jsonReq.Params) < 1 {
		return false
	}

	addr := strings.ToLower(r.jsonReq.Params[0].(string))
	_, shouldInterceptNonce := State.accountWithNonceFix[addr]
	if !shouldInterceptNonce {
		return false
	}

	// Cleanup (send wrong nonce at most 1x)
	delete(State.accountWithNonceFix, addr)

	// Do nothing if older than 1h
	// if time.Since(timeAdded).Hours() >= 1 {
	// 	return false
	// }

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
