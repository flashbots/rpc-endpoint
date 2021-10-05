package server

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
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
	respw.Header().Set("Access-Control-Allow-Origin", "*")
	respw.Header().Set("Access-Control-Allow-Headers", "Accept,Content-Type")

	// Serve a static file if the user is in a browser
	if req.Method == "GET" {
		http.ServeFile(respw, req, "./public/index.html")
		return
	}

	if req.Method == "OPTIONS" {
		respw.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("Number of Go-routines %d", runtime.NumGoroutine())

	// No blacklisted IPs for now
	ip := GetIP(req)
	if IsBlacklisted(ip) {
		log.Printf("Blocked: IP=%s", ip)
		respw.WriteHeader(http.StatusUnauthorized)
		return
	}

	// If users specify a proxy url in their rpc endpoint they can have their requests proxied to that endpoint instead of Infura
	// e.g. https://rpc.flashbots.net?url=http://RPC-ENDPOINT.COM
	url := r.ProxyUrl
	if len(req.URL.String()) >= 6 {
		// Debug
		log.Println(req.URL.String())
		url = req.URL.String()[6:]
	}

	log.Println("Using url:", url)

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
		log.Printf("ERROR: failed to read request body: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()
	log.Printf("[debug] Received: IP=%s", ip)
	// log.Printf("[debug] Received: IP=%s Header=%v", ip, req.Header)
	// log.Printf("[debug] Received: IP=%s Body=%s Header=%v", ip, string(body), req.Header)

	// Parse JSON RPC:
	var jsonReq *JsonRpcRequest
	if err := json.Unmarshal(body, &jsonReq); err != nil {
		log.Printf("ERROR: failed to parse JSON RPC request: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}

	if jsonReq.Method == "eth_sendRawTransaction" {
		isOFACBlacklisted, err := r.TxRelayer.checkForOFACList(jsonReq)
		if err != nil {
			log.Printf("ERROR: failed to check transaction OFAC status: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}

		if isOFACBlacklisted {
			log.Printf("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
			respw.WriteHeader(http.StatusUnauthorized)
			return
		}

		needsProtection, err := r.TxRelayer.EvaluateTransactionForFrontrunningProtection(jsonReq)
		if err != nil {
			log.Printf("ERROR: failed to evaluate transaction: %v", err)
			respw.WriteHeader(http.StatusBadRequest)
			return
		}

		if needsProtection {
			log.Printf("Sending tx to Flashbots")
			// Evaluated that this transaction needs protection and should be relayed
			jsonResp, err := r.TxRelayer.SendToTxManager(jsonReq)
			if err != nil {
				log.Printf("ERROR: failed to relay tx: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := json.NewEncoder(respw).Encode(jsonResp); err != nil {
				log.Printf("ERROR: failed to encode JSON RPC: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
			}
			log.Printf("Successfully relayed %s", jsonReq.Method)
			return
		} else {
			log.Printf("Sending tx to mempool")
			// Evaluated that this transaction does not need protection and can be sent to the mempool
			jsonResp, err := r.TxRelayer.SendTransactionToMempool(jsonReq, url)
			if err != nil {
				log.Printf("ERROR: failed to relay tx: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := json.NewEncoder(respw).Encode(jsonResp); err != nil {
				log.Printf("ERROR: failed to encode JSON RPC: %v", err)
				respw.WriteHeader(http.StatusBadRequest)
			}
			log.Printf("Successfully relayed %s", jsonReq.Method)
			return
		}
	}

	// Proxy request:
	// log.Printf("url: %v", url)
	// log.Printf("body: %v", body)

	// Non-eth_sendRawTransaction requests go through ProxyUrl:
	proxyResp, err := ProxyRequest(url, body)
	log.Printf("resp: %v", proxyResp)
	if err != nil {
		log.Printf("ERROR: failed to make proxy request: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
	if err != nil {
		log.Printf("ERROR: failed to read proxy response: %v", err)
		respw.WriteHeader(http.StatusBadRequest)
		return
	}
	defer proxyResp.Body.Close()

	respw.WriteHeader(proxyResp.StatusCode)
	respw.Write(proxyRespBody)
	// log.Printf("Successfully proxied %s. Result: %v", jsonReq.Method, string(proxyRespBody))
	log.Printf("Successfully proxied %s", jsonReq.Method)
	// log.Printf("Successfully relayed %s. Result: %+v", jsonReq.Method, jsonResp)
}

func IsBlacklisted(ip string) bool {
	for i := range blacklistedIps {
		if strings.HasPrefix(ip, blacklistedIps[i]) {
			return true
		}
	}
	return false
}
