package main

import (
	"flag"
	"os"

	"github.com/flashbots/rpc-endpoint/server"
)

var (
	defaultListenAddress = "127.0.0.1:9000"
	defaultProxyUrl      = "http://127.0.0.1:8545"
	defaultTxManagerUrl  = "https://protection.flashbots.net/v1/rpc"
)

var listenAddress = flag.String("listen", getEnvOrDefault("LISTEN_ADDR", defaultListenAddress), "Listen address")
var proxyUrl = flag.String("proxy", getEnvOrDefault("PROXY_URL", defaultProxyUrl), "URL for proxy eth_call-like request")
var txManagerUrl = flag.String("txmgr", getEnvOrDefault("TX_MANAGER_URL", defaultTxManagerUrl), "URL for tx manager")

func main() {
	flag.Parse()
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
