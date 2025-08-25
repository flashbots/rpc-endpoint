package server

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flashbots/rpc-endpoint/types"
)

func (r *RpcRequest) handle_sendPrivateTransaction() {
	if len(r.jsonReq.Params) > 1 {
		m, ok := r.jsonReq.Params[1].(string)
		if !ok {
			r.writeRpcError("MaxBlockNumber must be a string", types.JsonRpcParseError)
			return
		}
		max, err := hexutil.DecodeUint64(m)
		if err != nil {
			r.writeRpcError("MaxBlockNumber must be a valid hexadecimal string", types.JsonRpcParseError)
			return
		}
		r.maxBlockNumberOverride = max
	}
	if len(r.jsonReq.Params) > 2 {
		f, ok := r.jsonReq.Params[2].(map[string]interface{})
		if !ok {
			r.writeRpcError("Preferences must be an object", types.JsonRpcParseError)
			return
		}
		fast, ok := f["fast"].(bool)
		if !ok {
			r.writeRpcError("Preferences fast must be a boolean", types.JsonRpcParseError)
			return
		}
		r.urlParams.pref.Fast = fast
	}

	r.handle_sendRawTransaction()
}
