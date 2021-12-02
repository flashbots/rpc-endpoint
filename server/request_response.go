package server

import (
	"encoding/json"
	"net/http"

	"github.com/flashbots/rpc-endpoint/types"
)

func (r *RpcRequest) writeHeaderStatus(statusCode int) {
	if r.respHeaderStatusCodeWritten {
		return
	}
	r.respHeaderStatusCodeWritten = true
	(*r.respw).WriteHeader(statusCode)
}

func (r *RpcRequest) writeHeaderContentType(contentType string) {
	if r.respHeaderStatusCodeWritten {
		r.logError("writeHeaderContentType failed because status code was already written")
	}
	if r.respHeaderContentTypeWritten {
		return
	}
	r.respHeaderContentTypeWritten = true
	(*r.respw).Header().Set("Content-Type", contentType)
}

func (r *RpcRequest) writeHeaderContentTypeJson() {
	r.writeHeaderContentType("application/json")
}

func (r *RpcRequest) writeRpcError(msg string) {
	res := types.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Error: &types.JsonRpcError{
			Code:    -32603,
			Message: msg,
		},
	}
	r._writeRpcResponse(&res)
}

func (r *RpcRequest) writeRpcResult(result interface{}) {
	resBytes, err := json.Marshal(result)
	if err != nil {
		r.logError("writeRpcResult error marshalling %s: %s", result, err)
		r.writeHeaderStatus(http.StatusInternalServerError)
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
	if r.respBodyWritten {
		r.logError("_writeRpcResponse: response already written")
		return
	}

	if !r.respHeaderContentTypeWritten {
		r.writeHeaderContentTypeJson() // set content type to json, if not yet set
	}

	if !r.respHeaderStatusCodeWritten {
		r.writeHeaderStatus(http.StatusOK) // set status header to 200, if not yet set
	}

	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.logError("failed writing rpc response: %v", err)
		r.writeHeaderStatus(http.StatusInternalServerError)
	}

	r.respBodyWritten = true
}
