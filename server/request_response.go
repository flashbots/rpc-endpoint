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

	// Choose httpStatusCode based on json-rpc error code
	statusCode := http.StatusOK

	if res.Error != nil {
		statusCode = rpcToHttpCode(res.Error.Code)
	}
	(*r.respw).WriteHeader(statusCode)

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

func rpcToHttpCode(rpcStatusCode int) int {
	statusCode := http.StatusOK
	switch rpcStatusCode {
	case types.JsonRpcInvalidRequest, types.JsonRpcInvalidParams:
		statusCode = http.StatusBadRequest
	case types.JsonRpcMethodNotFound:
		statusCode = http.StatusNotFound
	case types.JsonRpcInternalError, types.JsonRpcParseError:
		statusCode = http.StatusInternalServerError
	default:
		statusCode = http.StatusInternalServerError
	}
	return statusCode
}
