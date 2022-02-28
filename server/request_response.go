package server

import (
	"encoding/json"
	"github.com/flashbots/rpc-endpoint/types"
	"net/http"
)

func (r *RpcRequestHandler) writeHeaderContentTypeJson() {
	(*r.respw).Header().Set("Content-Type", "application/json")
}

func (r *RpcRequestHandler) _writeRpcResponse(res *types.JsonRpcResponse) {
	// If the request is single and not batch
	// Write content type
	r.writeHeaderContentTypeJson() // Set content type to json
	(*r.respw).WriteHeader(http.StatusOK)
	// Write response
	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.logger.Error("[_writeRpcResponse] Failed writing rpc response", "error", err)
		(*r.respw).WriteHeader(http.StatusInternalServerError)
	}
}

func (r *RpcRequestHandler) _writeRpcBatchResponse(res []*types.JsonRpcResponse) {
	r.writeHeaderContentTypeJson() // Set content type to json
	(*r.respw).WriteHeader(http.StatusOK)
	// Write response
	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.logger.Error("[_writeRpcBatchResponse] Failed writing rpc response", "error", err)
		(*r.respw).WriteHeader(http.StatusInternalServerError)
	}
}
