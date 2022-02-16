/*
 * Test helpers.
 */
package testutils

import (
	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/flashbots/rpc-endpoint/types"
)

var RpcEndpointUrl string // set by tests

func SendRpcAndParseResponse(req *types.JsonRpcRequest) (*types.JsonRpcResponse, error) {
	return SendRpcAndParseResponseTo(RpcEndpointUrl, req)
}

func SendBatchRpcAndParseResponse(req []*types.JsonRpcRequest) ([]*types.JsonRpcResponse, error) {
	return SendBatchRpcAndParseResponseTo(RpcEndpointUrl, req)
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

func SendRpcAndParseResponseTo(url string, req *types.JsonRpcRequest) (*types.JsonRpcResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "marshal")
	}

	// fmt.Printf("%s\n", jsonData)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "post")
	}

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read")
	}

	jsonRpcResp := new(types.JsonRpcResponse)

	// Check if returned an error, if so then convert to standard JSON-RPC error
	errorResp := new(types.RelayErrorResponse)
	if err := json.Unmarshal(respData, errorResp); err == nil && errorResp.Error != "" {
		// relay returned an error, convert to standard JSON-RPC error now
		jsonRpcResp.Error = &types.JsonRpcError{Message: errorResp.Error}
		return jsonRpcResp, nil
	}

	// Unmarshall JSON-RPC response and check for error inside
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	return jsonRpcResp, nil
}

func SendBatchRpcAndParseResponseTo(url string, req []*types.JsonRpcRequest) ([]*types.JsonRpcResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "marshal")
	}

	// fmt.Printf("%s\n", jsonData)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "post")
	}

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read")
	}

	var jsonRpcResp []*types.JsonRpcResponse

	// Unmarshall JSON-RPC response and check for error inside
	if err := json.Unmarshal(respData, &jsonRpcResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	return jsonRpcResp, nil
}
