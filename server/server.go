package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

var Now = time.Now // used to mock time in tests

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

// Transactions should only be sent once to the relay
var txForwardedToRelay map[string]time.Time = make(map[string]time.Time)

// Metamask fix helper
var MetaMaskFix = NewMetaMaskFixer()

type RpcEndPointServer struct {
	listenAddress string
	proxyUrl      string
	txManagerUrl  string
	relayUrl      string
	useRelay      bool
}

func NewRpcEndPointServer(listenAddress, proxyUrl, txManagerUrl string, relayUrl string, useRelay bool) *RpcEndPointServer {
	return &RpcEndPointServer{
		listenAddress: listenAddress,
		proxyUrl:      proxyUrl,
		txManagerUrl:  txManagerUrl,
		relayUrl:      relayUrl,
		useRelay:      useRelay,
	}
}

func (r *RpcEndPointServer) Start() {
	log.Printf("Starting rpc endpoint at %v (using relay %v)...", r.listenAddress, r.useRelay)

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.HandleFunc("/", http.HandlerFunc(r.HandleHttpRequest))
	http.HandleFunc("/health", http.HandlerFunc(r.handleHealthRequest))

	// Start serving
	if err := http.ListenAndServe(r.listenAddress, nil); err != nil {
		log.Fatalf("Failed to start rpc endpoint: %v", err)
	}
}

func (r *RpcEndPointServer) HandleHttpRequest(respw http.ResponseWriter, req *http.Request) {
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

	request := NewRpcRequest(&respw, req, r.proxyUrl, r.txManagerUrl, r.relayUrl, r.useRelay)
	request.process()
}

func (r *RpcEndPointServer) handleHealthRequest(respw http.ResponseWriter, req *http.Request) {
	respw.WriteHeader(http.StatusOK)
	msg := fmt.Sprintf("All systems OK: %s", time.Now().UTC())
	io.WriteString(respw, msg)
}

func IsBlacklisted(ip string) bool {
	for i := range blacklistedIps {
		if strings.HasPrefix(ip, blacklistedIps[i]) {
			return true
		}
	}
	return false
}
