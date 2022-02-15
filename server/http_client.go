package server

import (
	"bytes"
	"net/http"
	"strconv"
	"time"
)

type HttpClient interface {
	ProxyRequest(body []byte) (*http.Response, error)
}

type httpClient struct {
	httpClient *http.Client // http client for making proxy request
	proxyURL   string       // Proxies the incoming request to the target URL
}

func NewHttpClient(proxyURL string) HttpClient {
	return &httpClient{
		httpClient: &http.Client{Timeout: time.Second * 5}, // default timeout set to 5s
		proxyURL:   proxyURL,
	}
}

// ProxyRequest using http client to make http post request
func (n *httpClient) ProxyRequest(body []byte) (*http.Response, error) {

	// create http request
	req, err := http.NewRequest(http.MethodPost, n.proxyURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))

	// make post request
	return n.httpClient.Do(req)
}
