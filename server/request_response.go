package server

import (
	"encoding/json"
	"net/http"

	"github.com/flashbots/rpc-endpoint/types"
)

func (r *RpcRequest) writeHeaderContentTypeJson() {
	(*r.respw).Header().Set("Content-Type", "application/json")
}

func (r *RpcRequest) _writeHeaderStatus(statusCode int) {
	(*r.respw).WriteHeader(statusCode)
}

func (r *RpcRequest) writeRpcError(msg string, errCode int) {
	res := types.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Error: &types.JsonRpcError{
			Code:    errCode,
			Message: msg,
		},
	}
	r._writeRpcResponse(&res)
}

func (r *RpcRequest) writeRpcResult(result interface{}) {
	resBytes, err := json.Marshal(result)
	if err != nil {
		r.logError("writeRpcResult error marshalling %s: %s", result, err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return
	}
	res := types.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Result:  resBytes,
	}
	r._writeRpcResponse(&res)
}

func (r *RpcRequest) _writeRpcResponse(res *types.JsonRpcResponse) {

	// If the request is batch, handle it in the end
	// when all the individual request gets completed
	if r.handleBatch {
		r.jsonRes = res
		return
	}

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
		r.logError("failed writing rpc response: %v", err)
		r._writeHeaderStatus(http.StatusInternalServerError)
	}
}

func (r *RpcRequest) _writeRpcBatchResponse(res []*types.JsonRpcResponse) {

	r.writeHeaderContentTypeJson()      // Set content type to json
	r._writeHeaderStatus(http.StatusOK) // Set status header to 200
	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.logError("failed writing rpc response: %v", err)
		r._writeHeaderStatus(http.StatusInternalServerError)
	}

}
