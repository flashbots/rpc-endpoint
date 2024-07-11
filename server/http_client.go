package server

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

type RPCProxyClient interface {
	ProxyRequest(body []byte) (*http.Response, error)
}

type rpcProxyClient struct {
	logger      log.Logger
	httpClient  http.Client
	proxyURL    string
	fingerprint Fingerprint
}

func NewRPCProxyClient(logger log.Logger, proxyURL string, timeoutSeconds int, fingerprint Fingerprint) RPCProxyClient {
	return &rpcProxyClient{
		logger:      logger,
		httpClient:  http.Client{Timeout: time.Second * time.Duration(timeoutSeconds)},
		proxyURL:    proxyURL,
		fingerprint: fingerprint,
	}
}

// ProxyRequest using http client to make http post request
func (n *rpcProxyClient) ProxyRequest(body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, n.proxyURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	if n.fingerprint != 0 {
		req.Header.Set(
			"X-Forwarded-For",
			n.fingerprint.ToIPv6().String(),
		)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	start := time.Now()
	res, err := n.httpClient.Do(req)
	n.logger.Info("[ProxyRequest] completed", "timeNeeded", time.Since(start))
	return res, err
}
