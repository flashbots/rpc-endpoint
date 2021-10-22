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

	version = "dev" // is set during build process
)

var listenAddress = flag.String("listen", getEnvOrDefault("LISTEN_ADDR", defaultListenAddress), "Listen address")
var proxyUrl = flag.String("proxy", getEnvOrDefault("PROXY_URL", defaultProxyUrl), "URL for proxy eth_call-like request")
var txManagerUrl = flag.String("txmgr", getEnvOrDefault("TX_MANAGER_URL", defaultTxManagerUrl), "URL for tx manager")
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
	s := server.NewRpcEndPointServer(*listenAddress, *proxyUrl, *txManagerUrl)
	s.Start()
}

func getEnvOrDefault(key string, defaultValue string) string {
	ret := os.Getenv(key)
	if ret == "" {
		ret = defaultValue
	}
	return ret
}
