package server

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

const TxManagerUrl = "https://protection.flashbots.net/v1/rpc"

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

type RpcEndPointServer struct {
	ListenAddress string
	ProxyUrl      string
}

func NewRpcEndPointServer(listenAddress string, proxyUrl string) *RpcEndPointServer {
	return &RpcEndPointServer{
		ListenAddress: listenAddress,
		ProxyUrl:      proxyUrl,
	}
}

func (r *RpcEndPointServer) Start() {
	log.Printf("Starting rpc endpoint at %v...", r.ListenAddress)

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.HandleFunc("/", http.HandlerFunc(r.handleHttpRequest))

	// Start serving
	if err := http.ListenAndServe(r.ListenAddress, nil); err != nil {
		log.Fatalf("Failed to start rpc endpoint: %v", err)
	}
}

func (r *RpcEndPointServer) handleHttpRequest(respw http.ResponseWriter, req *http.Request) {
	timeRequestStart := time.Now() // for measuring execution time

	requestId := uuid.New().String()

	// rLog for request-log (it prefixes logs with the request id)
	rLog := func(format string, v ...interface{}) {
		ReqLog(requestId, format, v...)
	}

	defer func() {
		timeRequestNeeded := time.Since(timeRequestStart)
		rLog("request took %.6f sec", timeRequestNeeded.Seconds())
	}()

	respw.Header().Set("Access-Control-Allow-Origin", "*")
	respw.Header().Set("Access-Control-Allow-Headers", "Accept,Content-Type")

	if req.Method == "GET" {
		http.Redirect(respw, req, "https://docs.flashbots.net/flashbots-protect/rpc/quick-start/", http.StatusFound)
		return
	}

	if req.Method == "OPTIONS" {
		respw.WriteHeader(http.StatusOK)
		return
	}

	ip := GetIP(req)
	rLog("POST request from ip: %s - goroutines: %d", ip, runtime.NumGoroutine())

	if IsBlacklisted(ip) {
		rLog("Blocked: IP=%s", ip)
		respw.WriteHeader(http.StatusUnauthorized)
		return
	}

	// If users specify a proxy url in their rpc endpoint they can have their requests proxied to that endpoint instead of Infura
	// e.g. https://rpc.flashbots.net?url=http://RPC-ENDPOINT.COM
	url := r.ProxyUrl
	if len(req.URL.String()) >= 6 {
		// Debug
		url = req.URL.String()[6:]
		rLog("Using custom url:", url)
	}

	// Currently commented out because this check only supports Chrome MetaMask.
	// We need to add support for other common browsers / wallets if we would like to support them.
	// if !IsMetamask(req) {
	// 	log.Printf("Blocked non-Metamask request")
	// 	respw.WriteHeader(http.StatusUnauthorized)
	// 	return
	// }

	// Read request body:
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rLog("ERROR: failed to read request body: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Parse JSON RPC
	var jsonReq *JsonRpcRequest
	if err := json.Unmarshal(body, &jsonReq); err != nil {
		rLog("ERROR: failed to parse JSON RPC request: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}

	rLog("JSON-RPC method: %s ip: %s", jsonReq.Method, ip)

	needsProtection := false
	if jsonReq.Method == "eth_sendRawTransaction" {
		if len(jsonReq.Params) < 1 {
			rLog("ERROR: no params for eth_sendRawTransaction")
			respw.WriteHeader(http.StatusBadRequest)
			return
		}

		txFrom, err := GetSenderFromRawTx(jsonReq.Params[0].(string))
		if err != nil {
			rLog("ERROR: couldn't get address from rawTx: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}

		if isOnOFACList(txFrom) {
			rLog("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
			respw.WriteHeader(http.StatusUnauthorized)
			return
		}

		if needsProtection, err = EvaluateTransactionForFrontrunningProtection(requestId, jsonReq); err != nil {
			rLog("ERROR: failed to evaluate transaction: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}

		if needsProtection {
			url = TxManagerUrl
			rLog("eth_sendRawTransaction: sending tx to Flashbots")
		}
	}

	// Non-eth_sendRawTransaction requests go through proxy:
	timeProxyStart := time.Now() // for measuring execution time
	rLog("proxy to: %s", url)
	proxyResp, err := ProxyRequest(url, body)
	timeProxyNeeded := time.Since(timeProxyStart)
	rLog("proxy response after %.6f: %v", timeProxyNeeded.Seconds(), proxyResp)
	if err != nil {
		rLog("ERROR: failed to make proxy request: %v", err)
		respw.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = writeProxyResponseToRequest(&respw, proxyResp)
	if err != nil {
		rLog("ERROR writing proxy response to user request: %v", err)
		respw.WriteHeader(http.StatusInternalServerError)
	}

	if needsProtection {
		rLog("successfully relayed to Flashbots")
	}
}

func IsBlacklisted(ip string) bool {
	for i := range blacklistedIps {
		if strings.HasPrefix(ip, blacklistedIps[i]) {
			return true
		}
	}
	return false
}

func writeProxyResponseToRequest(respw *http.ResponseWriter, proxyResp *http.Response) error {
	proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
	if err != nil {
		return err
	}
	defer proxyResp.Body.Close()

	// Write
	(*respw).WriteHeader(proxyResp.StatusCode)
	_, err = (*respw).Write(proxyRespBody)
	return err
}
