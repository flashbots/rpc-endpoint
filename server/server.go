package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/alicebob/miniredis"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/metachris/flashbotsrpc"
	"github.com/pkg/errors"
)

var Now = time.Now // used to mock time in tests

var DebugDontSendTx = os.Getenv("DEBUG_DONT_SEND_RAWTX") != ""

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

var maxBundleCacheKeys = uint64(2000)

// Metamask fix helper
var RState *RedisState

var FlashbotsRPC *flashbotsrpc.FlashbotsRPC

func init() {
	log.SetOutput(os.Stdout)
}

type RpcEndPointServer struct {
	version         string
	startTime       time.Time
	listenAddress   string
	proxyUrl        string
	relaySigningKey *ecdsa.PrivateKey
	allowTxCache    bool
}

func NewRpcEndPointServer(version string, listenAddress, proxyUrl, relayUrl string, relaySigningKey *ecdsa.PrivateKey, redisUrl string) (*RpcEndPointServer, error) {
	var err error

	if DebugDontSendTx {
		log.Println("DEBUG MODE: raw transactions will not be sent out!")
	}

	if redisUrl == "dev" {
		log.Println("Using integrated in-memory Redis instance")
		redisServer, err := miniredis.Run()
		if err != nil {
			return nil, err
		}
		redisUrl = redisServer.Addr()
	}

	// Setup redis connection
	log.Println("Connecting to redis at", redisUrl, "...")
	RState, err = NewRedisState(redisUrl)
	if err != nil {
		return nil, errors.Wrap(err, "Redis init error")
	}

	FlashbotsRPC = flashbotsrpc.New(relayUrl)
	FlashbotsRPC.Debug = true

	return &RpcEndPointServer{
		startTime:       Now(),
		version:         version,
		listenAddress:   listenAddress,
		proxyUrl:        proxyUrl,
		relaySigningKey: relaySigningKey,
	}, nil
}

func (s *RpcEndPointServer) Start() {
	log.Printf("Starting rpc endpoint %s at %v...", s.version, s.listenAddress)

	// Start regular tasks
	s.SpawnRegularTasks()

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

	request := NewRpcRequest(&respw, req, s.proxyUrl, s.relaySigningKey, s.allowTxCache)
	request.process()
}

func (s *RpcEndPointServer) handleHealthRequest(respw http.ResponseWriter, req *http.Request) {
	res := types.HealthResponse{
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

func (s *RpcEndPointServer) SpawnRegularTasks() {
	// All 10 seconds: print some debug info
	go func() {
		for {
			log.Printf("num-goroutines: %d", runtime.NumGoroutine())
			time.Sleep(10 * time.Second)
		}
	}()

	// Every minute: check whether bundleCache is growing too large
	go func() {
		for {
			t1 := time.Now()
			numCachedKeys, err := RState.GetNumberOfBuncleCacheKeys()
			if err != nil {
				log.Println("[server.CheckBundleCache] ERROR in ", err)
			} else {
				log.Printf("[server.CheckBundleCache] num-keys: %d / time-needed: %.3f sec", numCachedKeys, time.Since(t1).Seconds())

				if numCachedKeys >= maxBundleCacheKeys && s.allowTxCache {
					log.Println("[server.CheckBundleCache] ERROR - Limit of cached keys reached, disabling caching now")
					s.allowTxCache = false
				} else if numCachedKeys < maxBundleCacheKeys && !s.allowTxCache {
					log.Println("[server.CheckBundleCache] Back below limit of cached keys reached, enableing caching again")
					s.allowTxCache = true
				}
			}
			time.Sleep(1 * time.Minute)
		}
	}()
}
