package server

import (
	"bytes"
	"github.com/ethereum/go-ethereum/log"
	"net/http"
	"strconv"
	"time"
)

type RPCProxyClient interface {
	ProxyRequest(body []byte) (*http.Response, error)
}

type rpcProxyClient struct {
	httpClient http.Client
	proxyURL   string
}

func NewRPCProxyClient(proxyURL string) RPCProxyClient {
	return &rpcProxyClient{
		httpClient: http.Client{Timeout: time.Second * 10},
		proxyURL:   proxyURL,
	}
}

// ProxyRequest using http client to make http post request
func (n *rpcProxyClient) ProxyRequest(body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, n.proxyURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	start := time.Now()
	res, err := n.httpClient.Do(req)
	log.Info("[ProxyRequest] after making ProxyRequest", "timeNeeded", time.Since(start))
	return res, err
}
