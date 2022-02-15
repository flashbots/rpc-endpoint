package server

import (
	"bytes"
	"net/http"
	"strconv"
	"time"
)

type RPCProxyClient interface {
	ProxyRequest(body []byte) (*http.Response, error)
}

type rpcProxyClient struct {
	httpClient http.Client // http client for making proxy request
	proxyURL   string      // target URL
}

func NewRPCProxyClient(proxyURL string) RPCProxyClient {
	return &rpcProxyClient{
		httpClient: http.Client{Timeout: time.Second * 5},
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
	return n.httpClient.Do(req)
}
