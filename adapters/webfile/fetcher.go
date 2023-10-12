package webfile

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

var ErrRequest = fmt.Errorf("request failed")

//https://raw.githubusercontent.com/flashbots/dowg/main/builder-registrations.json

type Fetcher struct {
	url string
	cl  http.Client
}

func NewFetcher(url string) *Fetcher {
	return &Fetcher{url: url, cl: http.Client{}}
}

func (f *Fetcher) Fetch(ctx context.Context) ([]byte, error) {
	//execute http request and load bytes
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.cl.Do(httpReq)
	if err != nil {
		return nil, err
	}
	bts, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("err: %w status code %d", ErrRequest, resp.StatusCode)
	}
	return bts, nil
}
