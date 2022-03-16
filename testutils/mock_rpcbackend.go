/*
 * Dummy RPC backend for both Ethereum node and Flashbots Relay.
 * Implements JSON-RPC calls that the tests need.
 */
package testutils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/flashbots/rpc-endpoint/types"
)

var getBundleStatusByTransactionHash_Response = types.GetBundleStatusByTransactionHashResponse{
	TxHash: TestTx_BundleFailedTooManyTimes_Hash,
	Status: "FAILED_BUNDLE",
}

var MockBackendLastRawRequest *http.Request
var MockBackendLastJsonRpcRequest *types.JsonRpcRequest
var MockBackendLastJsonRpcRequestTimestamp time.Time

func MockRpcBackendReset() {
	MockBackendLastRawRequest = nil
	MockBackendLastJsonRpcRequest = nil
	MockBackendLastJsonRpcRequestTimestamp = time.Time{}
}

func handleRpcRequest(req *types.JsonRpcRequest) (result interface{}, err error) {
	MockBackendLastJsonRpcRequest = req

	switch req.Method {
	case "eth_getTransactionCount":
		if req.Params[0] == TestTx_BundleFailedTooManyTimes_From {
			return TestTx_BundleFailedTooManyTimes_Nonce, nil
		} else if req.Params[0] == TestTx_CancelAtRelay_Cancel_From {
			return TestTx_CancelAtRelay_Cancel_Nonce, nil
		}
		return "0x22", nil

	case "eth_call":
		return "0x12345", nil

	case "eth_getTransactionReceipt":
		if req.Params[0] == TestTx_BundleFailedTooManyTimes_Hash {
			return nil, nil
		} else if req.Params[0] == TestTx_MM2_Hash {
			return nil, nil
		}

	case "eth_sendRawTransaction":
		txHash := req.Params[0].(string)
		if txHash == TestTx_CancelAtRelay_Cancel_RawTx {
			return TestTx_CancelAtRelay_Cancel_Hash, nil
		}
		return "tx-hash1", nil

	case "net_version":
		return "3", nil

	case "null":
		return nil, nil

		// Relay calls
	case "eth_sendPrivateTransaction":
		param := req.Params[0].(map[string]interface{})
		if param["tx"] == TestTx_BundleFailedTooManyTimes_RawTx {
			return TestTx_BundleFailedTooManyTimes_Hash, nil
		} else {
			return "tx-hash2", nil
		}

	case "eth_cancelPrivateTransaction":
		param := req.Params[0].(map[string]interface{})
		if param["txHash"] == TestTx_CancelAtRelay_Cancel_Hash {
			return true, nil
		} else {
			return false, nil
		}
	}

	return "", fmt.Errorf("no RPC method handler implemented for %s", req.Method)
}

func RpcBackendHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	MockBackendLastRawRequest = req
	MockBackendLastJsonRpcRequestTimestamp = time.Now()

	log.Printf("%s %s %s\n", req.RemoteAddr, req.Method, req.URL)

	w.Header().Set("Content-Type", "application/json")
	testHeader := req.Header.Get("Test")
	w.Header().Set("Test", testHeader)

	returnError := func(id interface{}, msg string) {
		log.Println("returnError:", msg)
		res := types.JsonRpcResponse{
			Id: id,
			Error: &types.JsonRpcError{
				Code:    -32603,
				Message: msg,
			},
		}

		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Printf("error writing response 1: %v - data: %s", err, res)
		}
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		returnError(-1, fmt.Sprintf("failed to read request body: %v", err))
		return
	}

	// Parse JSON RPC
	jsonReq := new(types.JsonRpcRequest)
	if err = json.Unmarshal(body, &jsonReq); err != nil {
		returnError(-1, fmt.Sprintf("failed to parse JSON RPC request: %v", err))
		return
	}

	rawRes, err := handleRpcRequest(jsonReq)
	if err != nil {
		returnError(jsonReq.Id, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	resBytes, err := json.Marshal(rawRes)
	if err != nil {
		fmt.Println("error mashalling rawRes:", rawRes, err)
	}

	res := types.NewJsonRpcResponse(jsonReq.Id, resBytes)

	// Write to client request
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("error writing response 2: %v - data: %s", err, rawRes)
	}
}
