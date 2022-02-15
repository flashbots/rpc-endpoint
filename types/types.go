package types

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"
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

type EthSendRawTransactionModel struct {
	Id                         uuid.UUID     `db:"id"`
	ReceivedAt                 time.Time     `db:"received_at"`
	InsertedAt                 time.Time     `db:"inserted_at"`
	RequestDuration            time.Duration `db:"request_duration"`
	RequestType                string        `db:"request_type"`
	HttpMethod                 string        `db:"http_method"`
	HttpUrl                    string        `db:"http_url"`
	HttpQueryParam             string        `db:"http_query_param"`
	HttpResponseStatus         int           `db:"http_response_status"`
	Ip                         string        `db:"ip"`
	Origin                     string        `db:"origin"`
	Host                       string        `db:"host"`
	IsOnOafcList               string        `db:"is_on_oafc_list"`
	IsWhiteHatBundleCollection bool          `db:"is_white_hat_bundle_collection"`
	WhiteHatBundleId           string        `db:"white_hat_bundle_id"`
	IsCancelTx                 bool          `db:"is_cancel_tx"`
	IsTxSentToRelay            bool          `db:"is_tx_sent_to_relay"`
	IsBlockedBczAlreadySent    bool          `db:"is_blocked_bcz_already_sent"`
	Error                      string        `db:"error"`
	TxRaw                      string        `db:"tx_raw"`
	TxHash                     string        `db:"tx_hash"`
	TxFrom                     string        `db:"tx_from"`
	TxTo                       string        `db:"tx_to"`
	TxNonce                    int           `db:"tx_nonce"`
	TxData                     string        `db:"tx_data"`
	TxSmartContractMethod      string        `db:"tx_smart_contract_method"`
}
