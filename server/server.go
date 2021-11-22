package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/pkg/errors"
)

var Now = time.Now // used to mock time in tests

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

// Metamask fix helper
var State = NewGlobalState()
var RState *RedisState

func init() {
	log.SetOutput(os.Stdout)
}

type RpcEndPointServer struct {
	version         string
	startTime       time.Time
	listenAddress   string
	proxyUrl        string
	relayUrl        string
	relaySigningKey *ecdsa.PrivateKey
}

func NewRpcEndPointServer(version string, listenAddress, proxyUrl, relayUrl string, relaySigningKey *ecdsa.PrivateKey, redisUrl string) (*RpcEndPointServer, error) {
	var err error

	// Setup redis connection
	RState, err = NewRedisState(redisUrl)
	if err != nil {
		return nil, errors.Wrap(err, "Redis init error")
	}

	return &RpcEndPointServer{
		startTime:       Now(),
		version:         version,
		listenAddress:   listenAddress,
		proxyUrl:        proxyUrl,
		relayUrl:        relayUrl,
		relaySigningKey: relaySigningKey,
	}, nil
}

func (s *RpcEndPointServer) Start() {
	log.Printf("Starting rpc endpoint %s at %v...", s.version, s.listenAddress)

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.HandleFunc("/", http.HandlerFunc(s.HandleHttpRequest))
	http.HandleFunc("/health", http.HandlerFunc(s.handleHealthRequest))

	// Start serving
	if err := http.ListenAndServe(s.listenAddress, nil); err != nil {
		log.Fatalf("Failed to start rpc endpoint: %v", err)
	}
}

func (s *RpcEndPointServer) HandleHttpRequest(respw http.ResponseWriter, req *http.Request) {
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

	request := NewRpcRequest(&respw, req, s.proxyUrl, s.relayUrl, s.relaySigningKey)
	request.process()
}

func (s *RpcEndPointServer) handleHealthRequest(respw http.ResponseWriter, req *http.Request) {
	res := HealthResponse{
		Now:       Now(),
		StartTime: s.startTime,
		Version:   s.version,
	}

	jsonResp, err := json.Marshal(res)
	if err != nil {
		log.Println("healthCheck json error:", err)
		respw.WriteHeader(http.StatusInternalServerError)
		return
	}

	respw.Header().Set("Content-Type", "application/json")
	respw.WriteHeader(http.StatusOK)
	respw.Write(jsonResp)
}

func IsBlacklisted(ip string) bool {
	for i := range blacklistedIps {
		if strings.HasPrefix(ip, blacklistedIps[i]) {
			return true
		}
	}
	return false
}
