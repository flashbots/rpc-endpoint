package application

import (
	"sync"
	"time"

	"github.com/flashbots/rpc-endpoint/types"
)

type value struct {
	data      *types.JsonRpcResponse
	timestamp int64
}
type RpcCache struct {
	mu    sync.Mutex
	cache map[string]value
	ttl   int64
}

func NewRpcCache(ttl int64) *RpcCache {
	return &RpcCache{
		cache: make(map[string]value),
		ttl:   ttl,
		mu:    sync.Mutex{},
	}
}

func (rc *RpcCache) Get(key string) (*types.JsonRpcResponse, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	v, ok := rc.cache[key]
	if !ok {
		return nil, false
	}
	if time.Now().Unix()-v.timestamp > rc.ttl {
		delete(rc.cache, key)
		return nil, false
	}
	return v.data, ok
}

func (rc *RpcCache) Set(key string, data *types.JsonRpcResponse) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache[key] = value{
		data:      data,
		timestamp: time.Now().Unix(),
	}
}
