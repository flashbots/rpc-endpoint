package server

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/pkg/errors"
	"net/url"
	"strconv"
	"strings"
)

var (
	DefaultAuctionHint = []string{"hash"}

	ErrEmptyHintQuery              = errors.New("Hint query must be non-empty if set.")
	ErrEmptyTargetBuilderQuery     = errors.New("Target builder query must be non-empty if set.")
	ErrIncorrectAuctionHints       = errors.New("Incorrect auction hint, must be one of: contract_address, function_selector, logs, calldata.")
	ErrIncorrectOriginId           = errors.New("Incorrect origin id, must be less then 255 char.")
	ErrIncorrectRefundQuery        = errors.New("Incorrect refund.")
	ErrIncorrectRefundAddressQuery = errors.New("Incorrect refundAddress.")
)

type URLParameters struct {
	pref       types.TxPrivacyPreferences
	prefWasSet bool
	originId   string
}

// ExtractParametersFromUrl extracts the auction preference from the url query
// If the auction preference is not set, it will default to disabled
// if no hints are set and auction is enabled we use default hints
func ExtractParametersFromUrl(url *url.URL) (params URLParameters, err error) {
	var hint []string
	hintQuery, ok := url.Query()["hint"]
	if ok {
		if len(hintQuery) == 0 {
			return params, ErrEmptyHintQuery
		}
		for _, hint := range hintQuery {
			// valid hints are: "hash", "contract_address", "function_selector", "logs", "calldata"
			if hint != "hash" && hint != "contract_address" && hint != "function_selector" && hint != "logs" && hint != "calldata" {
				return params, ErrIncorrectAuctionHints
			}
		}
		hint = hintQuery
		params.prefWasSet = true
	} else {
		hint = DefaultAuctionHint
	}
	// append special logs hidden hint, so we can leak swap logs for protect txs
	hint = append(hint, "special_logs")
	params.pref.Hints = hint

	originIdQuery, ok := url.Query()["originId"]
	if ok {
		if len(originIdQuery) == 0 {
			return params, ErrIncorrectOriginId
		}
		params.originId = originIdQuery[0]
	}

	targetBuildersQuery, ok := url.Query()["builder"]
	if ok {
		if len(targetBuildersQuery) == 0 {
			return params, ErrEmptyTargetBuilderQuery
		}
		params.pref.Builders = targetBuildersQuery
	}

	refundQuery, ok := url.Query()["refund"]
	if ok {
		if len(refundQuery) != 1 {
			return params, ErrIncorrectRefundQuery
		}
		// parse refund as int
		refund, err := strconv.Atoi(refundQuery[0])
		if err != nil {
			return params, ErrIncorrectRefundQuery
		}
		if refund < 0 || refund > 100 {
			return params, ErrIncorrectRefundQuery
		}
		params.pref.WantRefund = &refund
	}

	refundAddressQuery, ok := url.Query()["refundAddress"]
	if ok {
		if len(refundAddressQuery) == 0 {
			return params, ErrIncorrectRefundAddressQuery
		} else if len(refundAddressQuery) == 1 {
			if !common.IsHexAddress(refundAddressQuery[0]) {
				return params, ErrIncorrectRefundAddressQuery
			}
			address := common.HexToAddress(refundAddressQuery[0])
			params.pref.RefundConfig = []types.RefundConfig{{Address: address, Percent: 100}}
		} else {
			var refundConfig []types.RefundConfig
			for _, refundAddress := range refundAddressQuery {
				split := strings.Split(refundAddress, ":")
				if len(split) != 2 {
					return params, ErrIncorrectRefundAddressQuery
				}
				if !common.IsHexAddress(split[0]) {
					return params, ErrIncorrectRefundAddressQuery
				}
				address := common.HexToAddress(split[0])
				percent, err := strconv.Atoi(split[1])
				if err != nil {
					return params, ErrIncorrectRefundAddressQuery
				}
				refundConfig = append(refundConfig, types.RefundConfig{Address: address, Percent: percent})
			}
			params.pref.RefundConfig = refundConfig
		}
	}

	return params, nil
}

func AuctionPreferenceErrorToJSONRPCResponse(jsonReq *types.JsonRpcRequest, err error) *types.JsonRpcResponse {
	message := fmt.Sprintf("Invalid auction preference in the rpc endpoint url. %s", err.Error())
	return &types.JsonRpcResponse{
		Id:      jsonReq.Id,
		Error:   &types.JsonRpcError{Code: types.JsonRpcInvalidRequest, Message: message},
		Version: "2.0",
	}
}
