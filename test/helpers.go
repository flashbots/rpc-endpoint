/*
 * Test helpers.
 */
package test

import (
	"encoding/json"
	"testing"

	"github.com/flashbots/rpc-endpoint/server"
	"github.com/flashbots/rpc-endpoint/types"
)

// Test tx for bundle-failed-too-many-times MM1 fix
var TestTx_BundleFailedTooManyTimes_RawTx = "0x02f9019d011e843b9aca008477359400830247fa94def1c0ded9bec7f1a1670819833240f027b25eff88016345785d8a0000b90128d9627aa40000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000016345785d8a000000000000000000000000000000000000000000000000001394b63b2cbaea253a00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0000000000000000000000006b175474e89094c44da98b954eedeac495271d0f869584cd00000000000000000000000086003b044f70dac0abc80ac8957305b6370893ed0000000000000000000000000000000000000000000000d3443651ba615f6cd6c001a011a9f58ebe30aa679783b31793f897fdb603dd2ea086845723a22dae85ab2864a0090cf1fcce0f6e85da54f4eccf32a485d71a7d39bc0b43a53a9e64901c656230"
var TestTx_BundleFailedTooManyTimes_From = "0xc84edF69E78C0E9dE5ccFE4fB9017F6F7566787f"
var TestTx_BundleFailedTooManyTimes_Hash = "0xfb34b88cd77215867aa8e8ff0abc7060178b8fed6519a85d0b22853dfd5e9fec"

// Test tx for MM2 fix
var TestTx_MM2_RawTx = "0xf86980850e5b35485d8252089409f427f1bd2d7537a02812275d03be7747dbd68c859d64bd68008025a0a94ba415e4d7c517548551442828aa192f149a8a04554e452084ee0ea55ea013a015966c84d9d38779ce63454a3f38038b269544d70433bfa7db0f6a37034b8e93"
var TestTx_MM2_From = "0x7AaBc7915DF92a85E199DbB4B1D21E637e1a90A2"
var TestTx_MM2_Hash = "0xc543e2ad05cffdee95b984df20edd2e38e124c54461faa1276adc36e826588c9"

var RpcEndpointUrl string // set by test init()

func sendRpcAndParseResponse(req *types.JsonRpcRequest) (*types.JsonRpcResponse, error) {
	return server.SendRpcAndParseResponseTo(RpcEndpointUrl, req)
}

func sendRpcAndParseResponseOrFailNow(t *testing.T, req *types.JsonRpcRequest) *types.JsonRpcResponse {
	res, err := sendRpcAndParseResponse(req)
	if err != nil {
		t.Fatal("sendRpcAndParseResponse error:", err)
	}
	return res
}

func sendRpcAndParseResponseOrFailNowString(t *testing.T, req *types.JsonRpcRequest) string {
	var rpcResult string
	resp := sendRpcAndParseResponseOrFailNow(t, req)
	json.Unmarshal(resp.Result, &rpcResult)
	return rpcResult
}

func sendRpcAndParseResponseOrFailNowAllowRpcError(t *testing.T, req *types.JsonRpcRequest) *types.JsonRpcResponse {
	res, err := sendRpcAndParseResponse(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
