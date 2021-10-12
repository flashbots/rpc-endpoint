package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const _txManagerUrl = "https://protection.flashbots.net/v1/rpc"

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

type RpcEndPointServer struct {
	ListenAddress string
	ProxyUrl      string
	TxManagerUrl  string
}

func NewRpcEndPointServer(listenAddress string, proxyUrl string) *RpcEndPointServer {
	return &RpcEndPointServer{
		ListenAddress: listenAddress,
		ProxyUrl:      proxyUrl,
		TxManagerUrl:  _txManagerUrl,
	}
}

func (r *RpcEndPointServer) Start() {
	log.Printf("Starting rpc endpoint at %v...", r.ListenAddress)

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.HandleFunc("/", http.HandlerFunc(r.HandleHttpRequest))
	http.HandleFunc("/health", http.HandlerFunc(r.handleHealthRequest))

	// Start serving
	if err := http.ListenAndServe(r.ListenAddress, nil); err != nil {
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

	request := NewRpcRequest(&respw, req, r.ProxyUrl, r.TxManagerUrl)
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
