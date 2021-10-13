/*
 * Dummy RPC backend. Implements Ethereum JSON-RPC calls that the tests need.
 */
package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/flashbots/rpc-endpoint/server"
)

func handleRpcRequest(req *server.JsonRpcRequest) (result interface{}, err error) {
	switch req.Method {
	case "eth_getTransactionCount":
		return "0x22", nil

	case "eth_call":
		return "0x12345", nil

	case "eth_sendRawTransaction":
		if req.Params[0] == RawTxBundleFailedTooManyTimes {
			return "", fmt.Errorf("Bundle submitted has already failed too many times") //lint:ignore ST1005 we mimic the error from the protect tx manager
		} else {
			return "bundle-id-from-BE", nil
		}

	case "net_version":
		return "3", nil
	}

	return 18, fmt.Errorf("foo")
}

func RpcBackendHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	log.Printf("%s %s %s\n", req.RemoteAddr, req.Method, req.URL)

	w.Header().Set("Content-Type", "application/json")
	testHeader := req.Header.Get("Test")
	w.Header().Set("Test", testHeader)

	returnError := func(id interface{}, msg string) {
		log.Println("returnError:", msg)
		res := server.JsonRpcResponse{
			Id: id,
			Error: &server.JsonRpcError{
				Code:    -32603,
				Message: msg,
			},
		}

		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Printf("error writing response: %v", err)
		}
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		returnError(-1, fmt.Sprintf("failed to read request body: %v", err))
		return
	}

	// Parse JSON RPC
	jsonReq := new(server.JsonRpcRequest)
	if err = json.Unmarshal(body, &jsonReq); err != nil {
		returnError(-1, fmt.Sprintf("failed to parse JSON RPC request: %v", err))
		return
	}

	rawRes, err := handleRpcRequest(jsonReq)
	if err != nil {
		returnError(jsonReq.Id, err.Error())
		return
	}

	res := server.JsonRpcResponse{
		Id:     jsonReq.Id,
		Result: rawRes,
	}
	// Write to client request
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("error writing response: %v", err)
	}
}
