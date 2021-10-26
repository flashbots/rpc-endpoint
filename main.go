package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/flashbots/rpc-endpoint/server"
)

var (
	defaultListenAddress = "127.0.0.1:9000"
	defaultProxyUrl      = "http://127.0.0.1:8545"
	defaultTxManagerUrl  = "https://protection.flashbots.net/v1/rpc"
	defaultRelayUrl      = "https://relay.flashbots.net"

	version = "dev" // is set during build process
)

var listenAddress = flag.String("listen", getEnvOrDefault("LISTEN_ADDR", defaultListenAddress), "Listen address")
var proxyUrl = flag.String("proxy", getEnvOrDefault("PROXY_URL", defaultProxyUrl), "URL for default JSON-RPC proxy target (eth node, Infura, etc.)")
var txManagerUrl = flag.String("txmgr", getEnvOrDefault("TX_MANAGER_URL", defaultTxManagerUrl), "URL for tx manager")
var relayUrl = flag.String("relay", getEnvOrDefault("RELAY_URL", defaultRelayUrl), "URL for relay")
var useRelay = flag.Bool("useRelay", false, "Use relay instead of tx-manager")
var versionPtr = flag.Bool("version", false, "just print the program version")

func main() {
	flag.Parse()

	// Perhaps print only the version
	if *versionPtr {
		fmt.Printf("rpc-endpoint %s\n", version)
		return
	}

	// Start the endpoint
	log.Printf("rpc-endpoint %s\n", version)
	s := server.NewRpcEndPointServer(*listenAddress, *proxyUrl, *txManagerUrl, *relayUrl, *useRelay)
	s.Start()
}

func getEnvOrDefault(key string, defaultValue string) string {
	ret := os.Getenv(key)
	if ret == "" {
		ret = defaultValue
	}
	return ret
}
