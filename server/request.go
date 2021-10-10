/*
Request represents an incoming client request
*/
package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
)

// Functions that never need protection
var allowedFunctions = map[string]bool{
	"a9059cbb": true, // transfer
	"23b872dd": true, // transferFrom
	"095ea7b3": true, // approve
	"2e1a7d4d": true, // weth withdraw
	"d0e30db0": true, // weth deposit
	"f242432a": true, // safe transfer NFT
}

// Blacklist for certain rawTx strings from being forwarded to BE.
// tx are added to blacklist after BE responds with 'Bundle submitted has already failed too many times'
var blacklistedRawTx = make(map[string]time.Time) // key is the rawTxHex, value is time added

// Helper to stop MetaMask from retrying
var mmBlacklistedAccountAndNonce = make(map[string]*mmNonceHelper)

type mmNonceHelper struct {
	Nonce    uint64
	NumTries uint64
}

type RpcRequest struct {
	respw *http.ResponseWriter
	req   *http.Request

	uid             string
	timeStarted     time.Time
	defaultProxyUrl string

	// extracted during request lifecycle:
	body     []byte
	jsonReq  *JsonRpcRequest
	ip       string
	rawTxHex string
	tx       *types.Transaction
	txFrom   string
}

// map[data:0xb9dc7766 from:0x34ca25940dbb0ce7e95db1fed8f8acf2b0ee9b09 gas:0x2dc6c0 to:0xd30149eca3c00be8aa36799ccfe171d79b4155d9 value:0x0]
type ethCallRequest struct {
	data  string
	from  string
	gas   string
	to    string
	value string
}

func NewRpcRequest(respw *http.ResponseWriter, req *http.Request, proxyUrl string) *RpcRequest {
	return &RpcRequest{
		respw:           respw,
		req:             req,
		uid:             uuid.New().String(),
		timeStarted:     time.Now(),
		defaultProxyUrl: proxyUrl,
	}
}

func (r *RpcRequest) log(format string, v ...interface{}) {
	prefix := fmt.Sprintf("[%s] ", r.uid)
	log.Printf(prefix+format, v...)
}

func (r *RpcRequest) logError(format string, v ...interface{}) {
	prefix := fmt.Sprintf("[%s] ERROR: ", r.uid)
	log.Printf(prefix+format, v...)
}

func (r *RpcRequest) process() {
	var err error

	// At end of request, log the time it needed
	defer func() {
		timeRequestNeeded := time.Since(r.timeStarted)
		r.log("request took %.6f sec", timeRequestNeeded.Seconds())
	}()

	r.ip = GetIP(r.req)
	r.log("POST request from ip: %s - goroutines: %d", r.ip, runtime.NumGoroutine())

	if IsBlacklisted(r.ip) {
		r.log("Blocked: IP=%s", r.ip)
		(*r.respw).WriteHeader(http.StatusUnauthorized)
		return
	}

	// If users specify a proxy url in their rpc endpoint they can have their requests proxied to that endpoint instead of Infura
	// e.g. https://rpc.flashbots.net?url=http://RPC-ENDPOINT.COM
	customProxyUrl, ok := r.req.URL.Query()["url"]
	if ok && len(customProxyUrl[0]) > 1 {
		r.defaultProxyUrl = customProxyUrl[0]
		r.log("Using custom url: %s", r.defaultProxyUrl)
	}

	// Decode request JSON RPC
	defer r.req.Body.Close()
	r.body, err = ioutil.ReadAll(r.req.Body)
	if err != nil {
		r.logError("failed to read request body: %v", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse JSON RPC
	if err = json.Unmarshal(r.body, &r.jsonReq); err != nil {
		r.logError("failed to parse JSON RPC request: %v", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	r.log("JSON-RPC method: %s ip: %s", r.jsonReq.Method, r.ip)

	if r.jsonReq.Method == "eth_sendRawTransaction" {
		r.handle_sendRawTransaction()
	} else {
		if r.jsonReq.Method == "eth_getTransactionCount" && len(r.jsonReq.Params) > 0 { // intercept call if needed to prevent MM from spamming
			addr := strings.ToLower(r.jsonReq.Params[0].(string))
			mmHelperBlacklistEntry, mmHelperBlacklistEntryFound := mmBlacklistedAccountAndNonce[addr]
			if mmHelperBlacklistEntryFound {
				// MM should get nonce+1 four times to stop resending
				mmBlacklistedAccountAndNonce[addr].NumTries += 1
				if mmBlacklistedAccountAndNonce[addr].NumTries == 4 {
					delete(mmBlacklistedAccountAndNonce, addr)
				}

				// Prepare custom JSON-RPC response
				resp := JsonRpcResponse{
					Id:      r.jsonReq.Id,
					Version: "2.0",
					Result:  fmt.Sprintf("0x%x", mmHelperBlacklistEntry.Nonce+1),
				}

				// Write to client request
				if err := json.NewEncoder(*r.respw).Encode(resp); err != nil {
					r.logError("Intercepting eth_getTransactionCount failed: %v", err)
					(*r.respw).WriteHeader(http.StatusInternalServerError)
					return
				} else {
					r.log("Intercepting eth_getTransactionCount successful for %s", addr)
					return
				}
			}
		}
		if r.jsonReq.Method == "eth_call" && len(r.jsonReq.Params) > 0 {
			ethCallReq := r.jsonReq.Params[0].(map[string]interface{})
			addressTo := strings.ToLower(ethCallReq["to"].(string))

			if addressTo == "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a" {
				r.handle_eth_call_to_FlashRPC_Contract()
				return
			}

		}

		// Just proxy the request to a node
		if r.proxyRequest(r.defaultProxyUrl) {
			r.log("Proxy to node successful: %s", r.jsonReq.Method)
		} else {
			r.log("Proxy to node failed: %s", r.jsonReq.Method)
		}
	}
}

func (r *RpcRequest) handle_eth_call_to_FlashRPC_Contract() {
	resp := JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Result:  "0x0000000000000000000000000000000000000000000000000000000000000001",
	}

	if err := json.NewEncoder(*r.respw).Encode(resp); err != nil {
		r.logError("Intercepting eth_call failed: %v", err)
		(*r.respw).WriteHeader(http.StatusInternalServerError)
		return
	} else {
		r.log("Intercepting eth_call successful")
		return
	}
}

func (r *RpcRequest) handle_sendRawTransaction() {
	var err error

	// JSON-RPC sanity checks
	if len(r.jsonReq.Params) < 1 {
		r.logError("no params for eth_sendRawTransaction")
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	r.rawTxHex = r.jsonReq.Params[0].(string)
	if len(r.rawTxHex) < 2 {
		r.logError("invalid raw transaction (wrong length)")
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	r.log("rawTx: %s", r.rawTxHex)

	if _, isBlacklistedTx := blacklistedRawTx[r.rawTxHex]; isBlacklistedTx {
		r.logError("rawTx blocked because bundle failed too many times")
		(*r.respw).WriteHeader(http.StatusTooManyRequests)
		return
	}

	r.tx, err = GetTx(r.rawTxHex)
	if err != nil {
		r.logError("Error getting transaction object")
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	r.txFrom, err = GetSenderFromRawTx(r.tx)
	if err != nil {
		r.logError("couldn't get address from rawTx: %v", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	if isOnOFACList(r.txFrom) {
		r.log("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
		(*r.respw).WriteHeader(http.StatusUnauthorized)
		return
	}

	needsProtection, err := r.doesTxNeedFrontrunningProtection(r.tx)
	if err != nil {
		r.logError("failed to evaluate transaction: %v", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	target := "mempool"
	url := r.defaultProxyUrl
	if needsProtection {
		target = "Flashbots"
		url = TxManagerUrl
	}

	// Proxy now!
	proxySuccess := r.proxyRequest(url)

	// Log after proxying
	if proxySuccess {
		r.log("Proxy to %s successful: eth_sendRawTransaction", target)
	} else {
		r.log("Proxy to %s failed: eth_sendRawTransaction", target)
	}
}

func (r *RpcRequest) proxyRequest(proxyUrl string) (success bool) {
	timeProxyStart := time.Now() // for measuring execution time
	r.log("proxyRequest to: %s", proxyUrl)

	// Proxy request
	proxyResp, err := ProxyRequest(proxyUrl, r.body)

	// Afterwards, check time and result
	timeProxyNeeded := time.Since(timeProxyStart)
	r.log("proxy response %d after %.6f: %v", proxyResp.StatusCode, timeProxyNeeded.Seconds(), proxyResp)
	if err != nil {
		r.logError("failed to make proxy request: %v", err)
		(*r.respw).WriteHeader(http.StatusInternalServerError)
		return false
	}

	// Read body
	defer proxyResp.Body.Close()
	proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
	if err != nil {
		r.logError("failed to decode proxy request body: %v", err)
		(*r.respw).WriteHeader(http.StatusInternalServerError)
		return false
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(JsonRpcResponse)
	if err := json.Unmarshal(proxyRespBody, jsonRpcResp); err != nil {
		r.logError("failed decoding proxy json-rpc response: %v", err)
		(*r.respw).WriteHeader(http.StatusInternalServerError)
		return false
	}

	// If JSON-RPC had an error response, parse but still pass back to user
	if jsonRpcResp.Error != nil {
		r.handleProxyError(jsonRpcResp.Error)
	}

	// Write status code header and body back to user request
	(*r.respw).WriteHeader(proxyResp.StatusCode)
	_, err = (*r.respw).Write(proxyRespBody)
	if err != nil {
		r.logError("failed writing proxy response to user request: %v", err)
		return false
	}

	return true
}

func (r *RpcRequest) handleProxyError(rpcError *JsonRpcError) {
	r.log("proxy response json-rpc error: %s", rpcError.Error())

	if rpcError.Message == "Bundle submitted has already failed too many times" {
		blacklistedRawTx[r.rawTxHex] = time.Now()
		r.log("rawTx added to blocklist. entries: %d", len(blacklistedRawTx))

		// Cleanup old rawTx blacklist entries
		for key, entry := range blacklistedRawTx {
			if time.Since(entry) > 4*time.Hour {
				delete(blacklistedRawTx, key)
			}
		}

		// To prepare for MM retrying the transactions, we get the txCount and then return it +1 for next four tries
		nonce, err := eth_getTransactionCount(r.defaultProxyUrl, r.txFrom)
		if err != nil {
			r.logError("failed getting nonce: %s", err)
			return
		}
		// fmt.Println("NONCE", nonce, "for", r.txFrom)
		mmBlacklistedAccountAndNonce[strings.ToLower(r.txFrom)] = &mmNonceHelper{
			Nonce: nonce,
		}
	}
}

// Check if a request needs frontrunning protection. There are many transactions that don't need frontrunning protection,
// for example simple ERC20 transfers.
func (r *RpcRequest) doesTxNeedFrontrunningProtection(tx *types.Transaction) (bool, error) {
	gas := tx.Gas()
	r.log("[protect-check] gas: %v", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false, nil
	}

	data := hex.EncodeToString(tx.Data())
	r.log("[protect-check] data: %v", data)
	if len(data) == 0 {
		r.log("[protect-check] Data had a length of 0, but a gas greater than 21000. Sending cancellation tx to mempool.")
		return false, nil
	}

	if isOnFunctionWhiteList(data[0:8]) {
		return false, nil // function being called is on our whitelist and no protection needed
	} else {
		return true, nil // needs protection if not on whitelist
	}
}

func isOnFunctionWhiteList(data string) bool {
	if allowedFunctions[data[0:8]] {
		return true
	} else {
		return false
	}
}
