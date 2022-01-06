/*
 * Test helpers.
 */
package testutils

import (
	"encoding/json"
	"testing"

	"github.com/flashbots/rpc-endpoint/types"
	"github.com/flashbots/rpc-endpoint/utils"
)

var RpcEndpointUrl string // set by tests

func SendRpcAndParseResponse(req *types.JsonRpcRequest) (*types.JsonRpcResponse, error) {
	return utils.SendRpcAndParseResponseTo(RpcEndpointUrl, req)
}

func SendBatchRpcAndParseResponse(req []*types.JsonRpcRequest) ([]*types.JsonRpcResponse, error) {
	return utils.SendBatchRpcAndParseResponseTo(RpcEndpointUrl, req)
}

func SendRpcAndParseResponseOrFailNow(t *testing.T, req *types.JsonRpcRequest) *types.JsonRpcResponse {
	res, err := SendRpcAndParseResponse(req)
	if err != nil {
		t.Fatal("sendRpcAndParseResponse error:", err)
	}
	return res
}

func SendRpcAndParseResponseOrFailNowString(t *testing.T, req *types.JsonRpcRequest) string {
	var rpcResult string
	resp := SendRpcAndParseResponseOrFailNow(t, req)
	json.Unmarshal(resp.Result, &rpcResult)
	return rpcResult
}

func SendRpcAndParseResponseOrFailNowAllowRpcError(t *testing.T, req *types.JsonRpcRequest) *types.JsonRpcResponse {
	res, err := SendRpcAndParseResponse(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
