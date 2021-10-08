/*
Request represents an incoming client request
*/
package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
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

var blacklistedAccountAndNonces = make(map[string]NonceBlocknumber)

type NonceBlocknumber struct {
	Nonce uint64
	Block uint64
}

type RpcRequest struct {
	respw *http.ResponseWriter
	req   *http.Request

	uid         string
	timeStarted time.Time
	proxyUrl    string

	// extracted during request lifecycle:
	body     []byte
	jsonReq  *JsonRpcRequest
	ip       string
	rawTxHex string
	tx       *types.Transaction
}

func NewRpcRequest(respw *http.ResponseWriter, req *http.Request, proxyUrl string) *RpcRequest {
	return &RpcRequest{
		respw:       respw,
		req:         req,
		uid:         uuid.New().String(),
		timeStarted: time.Now(),
		proxyUrl:    proxyUrl,
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
		r.proxyUrl = customProxyUrl[0]
		r.log("Using custom url:", r.proxyUrl)
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
		// Just proxy the request to a node
		if r.proxyRequest() {
			r.log("Proxy to mempool successful: %s", r.jsonReq.Method)
		} else {
			r.log("Proxy to mempool failed: %s", r.jsonReq.Method)
		}
	}
}

func (r *RpcRequest) handle_sendRawTransaction() {
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

	tx, err := GetTx(r.rawTxHex)
	if err != nil {
		r.logError("Error getting transaction object")
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}
	r.tx = tx

	txFrom, err := GetSenderFromRawTx(r.tx)
	if err != nil {
		r.logError("couldn't get address from rawTx: %v", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	if isOnOFACList(txFrom) {
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
	if needsProtection {
		target = "Flashbots"
		r.proxyUrl = TxManagerUrl
	}

	// Proxy now!
	proxySuccess := r.proxyRequest()

	// Log after proxying
	if proxySuccess {
		r.log("Proxy to %s successful: eth_sendRawTransaction", target)
	} else {
		r.log("Proxy to %s failed: eth_sendRawTransaction", target)
	}
}

func (r *RpcRequest) proxyRequest() (success bool) {
	timeProxyStart := time.Now() // for measuring execution time
	r.log("proxyRequest to: %s", r.proxyUrl)

	// Proxy request
	proxyResp, err := ProxyRequest(r.proxyUrl, r.body)

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

func (r *RpcRequest) getNonce() (uint64, error) {
	txFrom, err := GetSenderFromRawTx(r.tx)
	if err != nil {
		r.logError("couldn't get address from rawTx: %v", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return 0, err
	}

	jsonData, err := json.Marshal(JsonRpcRequest{
		Id:      1,
		Version: "2.0",
		Method:  "eth_getTransactionCount",
		Params:  []interface{}{txFrom, "latest"},
	})

	if err != nil {
		r.logError("failed to marshal JSON RPC request: %v", err)
		return 0, err
	}

	// Execute eth_sendRawTransaction JSON-RPC request
	resp, err := http.Post(r.proxyUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		r.log("Error sending eth_getTransactionCount (sending request): %s", err)
		return 0, err
	}

	// Check response for errors
	r.log("resp: %v", resp)
	respData, err := ioutil.ReadAll(resp.Body)
	r.log("respData: %s", respData)
	if err != nil {
		r.log("Error sending tx (reading body): %s", err)
		return 0, err
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(JsonRpcResponse)
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		r.log("Error sending tx (decoding json rpc response): %s", err)
		return 0, err
	}

	if jsonRpcResp.Error != nil {
		r.log("json rpc response error: %s", *jsonRpcResp.Error)
		return 0, err
	}

	// getTransactionCount request here
	return 1, nil
}

func (r *RpcRequest) handleProxyError(rpcError *JsonRpcError) {
	r.log("proxy response json-rpc error: %s", rpcError.Error())
	if rpcError.Message == "Bundle submitted has already failed too many times" {
		blacklistedRawTx[r.rawTxHex] = time.Now()
		r.log("rawTx added to blocklist. entries: %d", len(blacklistedRawTx))

		from, err := GetSenderFromTx(r.tx)
		if err != nil {
			r.logError("Error deriving tx from")
		}
		nonce, err := r.getNonce()
		if err != nil {
			r.logError("Error getting nonce: %s", err)
		}
		blacklistedAccountAndNonces[from] = NonceBlocknumber{
			Nonce: nonce,
			Block: 0, // TBD
		}

		// Cleanup old entries
		for key, entry := range blacklistedRawTx {
			if time.Since(entry) > 4*time.Hour {
				delete(blacklistedRawTx, key)
			}
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
