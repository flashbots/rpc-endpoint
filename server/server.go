package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

type RpcEndPointServer struct {
	ListenAddress string
	ProxyUrl      string
	TxRelayer     *PrivateTxRelayer
}

func NewRpcEndPointServer(listenAddress string, proxyUrl string, txRelayer *PrivateTxRelayer) *RpcEndPointServer {
	return &RpcEndPointServer{
		ListenAddress: listenAddress,
		ProxyUrl:      proxyUrl,
		TxRelayer:     txRelayer,
	}
}

func (r *RpcEndPointServer) Start() {
	log.Printf("Starting rpc endpoint at %v...", r.ListenAddress)

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.Handle("/", http.HandlerFunc(r.handleHttpRequest))

	// Serve files from the local 'public' directory under the '/public/' URL
	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/public/", http.StripPrefix("/public/", fileServer))

	// Start serving
	if err := http.ListenAndServe(r.ListenAddress, nil); err != nil {
		log.Fatalf("Failed to start rpc endpoint: %v", err)
	}
}

func (r *RpcEndPointServer) handleHttpRequest(respw http.ResponseWriter, req *http.Request) {
	timeRequestStart := time.Now() // for measuring execution time

	requestId := uuid.New()
	rLog := func(format string, v ...interface{}) {
		prefix := fmt.Sprintf("[%s] ", requestId)
		log.Printf(prefix+format, v...)
	}

	defer func() {
		timeRequestNeeded := time.Since(timeRequestStart)
		rLog("request took %f.4 sec", timeRequestNeeded.Seconds())
	}()

	respw.Header().Set("Access-Control-Allow-Origin", "*")
	respw.Header().Set("Access-Control-Allow-Headers", "Accept,Content-Type")

	if req.Method == "GET" {
		http.ServeFile(respw, req, "./public/index.html")
		return
	}

	if req.Method == "OPTIONS" {
		respw.WriteHeader(http.StatusOK)
		return
	}

	rLog("POST request. goroutines=%d", runtime.NumGoroutine())

	// For now restrict to certain IPs:
	ip := GetIP(req)

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
		// rLog.Println(req.URL.String())
		url = req.URL.String()[6:]
		rLog("Using custom url:", url)
	}

	// log.Println("Using url:", url)

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
	// log.Printf("[debug] Received: IP=%s", ip)
	// log.Printf("[debug] Received: IP=%s Header=%v", ip, req.Header)
	// log.Printf("[debug] Received: IP=%s Body=%s Header=%v", ip, string(body), req.Header)

	// Parse JSON RPC:
	var jsonReq *JsonRpcRequest
	if err := json.Unmarshal(body, &jsonReq); err != nil {
		rLog("ERROR: failed to parse JSON RPC request: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}

	rLog("JSON-RPC method: %s", jsonReq.Method)

	if jsonReq.Method == "eth_sendRawTransaction" {
		isOFACBlacklisted, err := r.TxRelayer.checkForOFACList(jsonReq)
		if err != nil {
			rLog("ERROR: failed to check transaction OFAC status: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}

		if isOFACBlacklisted {
			rLog("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
			respw.WriteHeader(http.StatusUnauthorized)
			return
		}

		needsProtection, err := r.TxRelayer.EvaluateTransactionForFrontrunningProtection(jsonReq)
		if err != nil {
			rLog("ERROR: failed to evaluate transaction: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}

		if needsProtection {
			rLog("eth_sendRawTransaction: sending tx to Flashbots")
			// Evaluated that this transaction needs protection and should be relayed
			jsonResp, err := r.TxRelayer.SendToTxManager(jsonReq)
			if err != nil {
				rLog("ERROR: failed to relay tx to Flashbots: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := json.NewEncoder(respw).Encode(jsonResp); err != nil {
				rLog("ERROR: failed to encode JSON RPC: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
			}
			rLog("eth_sendRawTransaction: successfully relayed to Flashbots.")
			return
		} else {
			rLog("eth_sendRawTransaction: sending tx to mempool")
			// Evaluated that this transaction does not need protection and can be sent to the mempool
			jsonResp, err := r.TxRelayer.SendTransactionToMempool(jsonReq, url)
			if err != nil {
				rLog("ERROR: failed to relay tx to Mempool: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := json.NewEncoder(respw).Encode(jsonResp); err != nil {
				rLog("ERROR: failed to encode JSON RPC: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
			}
			rLog("eth_sendRawTransaction: successfully relayed to mempool.")
			return
		}
	}

	// Proxy request:
	// log.Printf("url: %v", url)
	// log.Printf("body: %v", body)

	// Non-eth_sendRawTransaction requests go through ProxyUrl:
	timeProxyStart := time.Now() // for measuring execution time
	rLog("proxy to: %s", url)
	proxyResp, err := ProxyRequest(url, body)
	timeProxyNeeded := time.Since(timeProxyStart)
	rLog("proxy response after %.4f: %v", timeProxyNeeded.Seconds(), proxyResp)
	if err != nil {
		rLog("ERROR: failed to make proxy request: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
	if err != nil {
		rLog("ERROR: failed to read proxy response: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	defer proxyResp.Body.Close()

	respw.WriteHeader(proxyResp.StatusCode)
	respw.Write(proxyRespBody)
	rLog("proxied finished for %s. Result: %v", jsonReq.Method, string(proxyRespBody))

	// log("Successfully relayed %s. Result: %+v", jsonReq.Method, jsonResp)
}

func IsBlacklisted(ip string) bool {
	for i := range blacklistedIps {
		if strings.HasPrefix(ip, blacklistedIps[i]) {
			return true
		}
	}
	return false
}
