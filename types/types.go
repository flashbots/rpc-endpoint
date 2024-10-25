package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// As per JSON-RPC 2.0 Specification
// https://www.jsonrpc.org/specification#error_object
const (
	JsonRpcParseError     = -32700
	JsonRpcInvalidRequest = -32600
	JsonRpcMethodNotFound = -32601
	JsonRpcInvalidParams  = -32602
	JsonRpcInternalError  = -32603
)

type JsonRpcRequest struct {
	Id      interface{}   `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Version string        `json:"jsonrpc,omitempty"`
}

func NewJsonRpcRequest(id interface{}, method string, params []interface{}) *JsonRpcRequest {
	return &JsonRpcRequest{
		Id:      id,
		Method:  method,
		Params:  params,
		Version: "2.0",
	}
}

func NewJsonRpcRequest1(id interface{}, method string, param interface{}) *JsonRpcRequest {
	return NewJsonRpcRequest(id, method, []interface{}{param})
}

type JsonRpcResponse struct {
	Id      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JsonRpcError   `json:"error,omitempty"`
	Version string          `json:"jsonrpc"`
}

// RpcError: https://www.jsonrpc.org/specification#error_object
type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err JsonRpcError) Error() string {
	return fmt.Sprintf("Error %d (%s)", err.Code, err.Message)
}

func NewJsonRpcResponse(id interface{}, result json.RawMessage) *JsonRpcResponse {
	return &JsonRpcResponse{
		Id:      id,
		Result:  result,
		Version: "2.0",
	}
}

type GetBundleStatusByTransactionHashResponse struct {
	TxHash            string `json:"txHash"`            // "0x0aeb9c61b342f7fc94a10d41c5d30a049a9cfa9ab764c6dd02204a19960ee567"
	Status            string `json:"status"`            // "FAILED_BUNDLE"
	Message           string `json:"message"`           // "Expired - The base fee was to low to execute this transaction, please try again"
	Error             string `json:"error"`             // "max fee per gas less than block base fee"
	BlocksCount       int    `json:"blocksCount"`       // 2
	ReceivedTimestamp int    `json:"receivedTimestamp"` // 1634568851003
	StatusTimestamp   int    `json:"statusTimestamp"`   // 1634568873862
}

type HealthResponse struct {
	Now       time.Time `json:"time"`
	StartTime time.Time `json:"startTime"`
	Version   string    `json:"version"`
}

type TransactionReceipt struct {
	TransactionHash string
	Status          string
}

type PrivateTxStatus string

var TxStatusUnknown PrivateTxStatus = "UNKNOWN"
var TxStatusPending PrivateTxStatus = "PENDING"
var TxStatusIncluded PrivateTxStatus = "INCLUDED"
var TxStatusFailed PrivateTxStatus = "FAILED"

type PrivateTxApiResponse struct {
	Status         PrivateTxStatus `json:"status"`
	Hash           string          `json:"hash"`
	MaxBlockNumber int             `json:"maxBlockNumber"`
}

type RelayErrorResponse struct {
	Error string `json:"error"`
}

type BundleResponse struct {
	BundleId string   `json:"bundleId"`
	RawTxs   []string `json:"rawTxs"`
}

type SendPrivateTxRequestWithPreferences struct {
	Tx             string                `json:"tx"`
	Preferences    *PrivateTxPreferences `json:"preferences,omitempty"`
	MaxBlockNumber uint64                `json:"maxBlockNumber"`
}

type TxPrivacyPreferences struct {
	Hints      []string `json:"hints"`
	Builders   []string `json:"builders"`
	UseMempool bool     `json:"useMempool"`
	MempoolRPC string   `json:"mempoolRpc"`
}

type TxValidityPreferences struct {
	Refund []RefundConfig `json:"refund,omitempty"`
}

type RefundConfig struct {
	Address common.Address `json:"address"`
	Percent int            `json:"percent"`
}

type PrivateTxPreferences struct {
	Privacy   TxPrivacyPreferences  `json:"privacy"`
	Validity  TxValidityPreferences `json:"validity"`
	Fast      bool                  `json:"fast"`
	CanRevert bool                  `json:"canRevert"`
}
