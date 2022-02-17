package server

import (
	"context"
	"encoding/json"
	"github.com/flashbots/rpc-endpoint/types"
	"net/http"
	"time"
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

	// msg and rpcStatusCode is used for request record
	var (
		msg           string
		rpcStatusCode int
	)

	if res.Error != nil {
		// TODO(Note): http.StatusUnauthorized is not mapped
		msg = res.Error.Message
		rpcStatusCode = res.Error.Code
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
	(*r.respw).WriteHeader(statusCode)

	// Write response
	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.updateRequestRecord(err.Error(), http.StatusInternalServerError, res.Error.Code)
		r.logger.Error("[_writeRpcResponse] Failed writing rpc response", "error", err)
		(*r.respw).WriteHeader(http.StatusInternalServerError)
	}
	r.updateRequestRecord(msg, statusCode, rpcStatusCode)
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

func (r *RpcRequestHandler) updateRequestRecord(msg string, httpStatusCode, rpcStatusCode int) {
	r.reqRecord.requestEntry.Id = r.uid
	r.reqRecord.UpdateRequestEntry(r.req, httpStatusCode, msg)
	if r.jsonReq.Method == "eth_sendRawTransaction" {
		r.reqRecord.ethSendRawTxEntry.RequestId = r.uid
		if rpcStatusCode != 0 {
			r.reqRecord.ethSendRawTxEntry.Error = msg
			r.reqRecord.ethSendRawTxEntry.ErrorCode = rpcStatusCode
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			r.reqRecord.SaveEthSendRawTxEntryToDB(ctx)
		}()
	}
}
