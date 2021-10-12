package test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/flashbots/rpc-endpoint/server"
	"github.com/pkg/errors"
)

const RawTxBundleFailedTooManyTimes = "0x02f9019d011e843b9aca008477359400830247fa94def1c0ded9bec7f1a1670819833240f027b25eff88016345785d8a0000b90128d9627aa40000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000016345785d8a000000000000000000000000000000000000000000000000001394b63b2cbaea253a00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0000000000000000000000006b175474e89094c44da98b954eedeac495271d0f869584cd00000000000000000000000086003b044f70dac0abc80ac8957305b6370893ed0000000000000000000000000000000000000000000000d3443651ba615f6cd6c001a011a9f58ebe30aa679783b31793f897fdb603dd2ea086845723a22dae85ab2864a0090cf1fcce0f6e85da54f4eccf32a485d71a7d39bc0b43a53a9e64901c656230"

var RpcEndpointUrl string // set by test init()

func newRpcRequest(method string, params []interface{}) *server.JsonRpcRequest {
	return &server.JsonRpcRequest{
		Id:      1,
		Method:  method,
		Params:  params,
		Version: "2.0",
	}
}

func sendRpcAndParseResponseTo(url string, req *server.JsonRpcRequest) (*server.JsonRpcResponse, error) {
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

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(server.JsonRpcResponse)
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	return jsonRpcResp, nil
}

func sendRpcAndParseResponse(req *server.JsonRpcRequest) (*server.JsonRpcResponse, error) {
	return sendRpcAndParseResponseTo(RpcEndpointUrl, req)
}

func sendRpcAndParseResponseOrFailNow(t *testing.T, req *server.JsonRpcRequest) *server.JsonRpcResponse {
	res, err := sendRpcAndParseResponse(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	return res
}

func sendRpcAndParseResponseOrFailNowAllowRpcError(t *testing.T, req *server.JsonRpcRequest) *server.JsonRpcResponse {
	res, err := sendRpcAndParseResponse(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
