package server

import (
	"encoding/json"
	"net/http"

	"github.com/flashbots/rpc-endpoint/types"
)

func (r *RpcRequestHandler) writeHeaderContentTypeJson() {
	(*r.respw).Header().Set("Content-Type", "application/json")
}

func (r *RpcRequestHandler) _writeHeaderStatus(statusCode int) {
	(*r.respw).WriteHeader(statusCode)
}

func (r *RpcRequestHandler) _writeRpcResponse(res *types.JsonRpcResponse) {

	// If the request is single and not batch
	// Write content type
	r.writeHeaderContentTypeJson() // Set content type to json

	// Choose httpStatusCode based on json-rpc error code
	statusCode := http.StatusOK
	if res.Error != nil {
		// TODO(Note): http.StatusUnauthorized is not mapped
		switch res.Error.Code {
		case types.JsonRpcInvalidRequest, types.JsonRpcInvalidParams:
			statusCode = http.StatusBadRequest
		case types.JsonRpcMethodNotFound:
			statusCode = http.StatusNotFound
		case types.JsonRpcInternalError, types.JsonRpcParseError:
			statusCode = http.StatusInternalServerError
		default:
			statusCode = http.StatusInternalServerError
		}
	}
	r._writeHeaderStatus(statusCode) // set status header

	// Write response
	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.logger.logError("failed writing rpc response: %v", err)
		r._writeHeaderStatus(http.StatusInternalServerError)
	}
}

func (r *RpcRequestHandler) _writeRpcBatchResponse(res []*types.JsonRpcResponse) {

	r.writeHeaderContentTypeJson()      // Set content type to json
	r._writeHeaderStatus(http.StatusOK) // Set status header to 200
	// Write response
	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.logger.logError("failed writing rpc response: %v", err)
		r._writeHeaderStatus(http.StatusInternalServerError)
	}

}
