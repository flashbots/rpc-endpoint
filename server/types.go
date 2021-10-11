package server

import "fmt"

type JsonRpcRequest struct {
	Id      interface{}   `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Version string        `json:"jsonrpc,omitempty"`
}

type JsonRpcResponse struct {
	Id      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JsonRpcError `json:"error,omitempty"`
	Version string        `json:"jsonrpc,omitempty"`
}

// RpcError: https://www.jsonrpc.org/specification#error_object
type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err JsonRpcError) Error() string {
	return fmt.Sprintf("Error %d (%s)", err.Code, err.Message)
}
