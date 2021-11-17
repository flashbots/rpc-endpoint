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

	txHash := r.jsonReq.Params[0].(string)

	// Make sure transaction was submitted before
	if _, txFound := MetaMaskFix.rawTransactionSubmission[strings.ToLower(txHash)]; !txFound {
		return
	}

	r.log("[MM2] eth_getTransactionReceipt is null for known tx %s", txHash)

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

	// call private-tx-api
	privTxApiUrl := fmt.Sprintf("%s/tx/%s", ProtectTxApiHost, txHash)
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

	respObj := new(PrivateTxApiResponse)
	err = json.Unmarshal(bodyBytes, respObj)
	if err != nil {
		r.log("[MM2] priv-tx-api response: %s", string(bodyBytes))
		r.logError("[MM2] privTxApi call json-unmarshal failed for %s (BE error): %s", txHash, err)
		return
	}

	r.log("[MM2] priv-tx-api status: %s", respObj.Status)
	if respObj.Status == "FAILED" {
		setMmNonceFix(txHash)
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
