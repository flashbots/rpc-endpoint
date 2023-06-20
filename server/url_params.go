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

	ErrEmptyHintQuery          = errors.New("Hint query must be non-empty if set.")
	ErrEmptyTargetBuilderQuery = errors.New("Target builder query must be non-empty if set.")
	ErrIncorrectAuctionHints   = errors.New("Incorrect auction hint, must be one of: contract_address, function_selector, logs, calldata.")
	ErrIncorrectOriginId       = errors.New("Incorrect origin id, must be less then 255 char.")
	ErrIncorrectRefundQuery    = errors.New("Incorrect refund.")
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

	refundAddressQuery, ok := url.Query()["refund"]
	if ok {
		if len(refundAddressQuery) == 0 {
			return params, ErrIncorrectRefundQuery
		}

		var (
			addresses = make([]common.Address, len(refundAddressQuery))
			percents  = make([]int, len(refundAddressQuery))
		)

		for i, refundAddress := range refundAddressQuery {
			split := strings.Split(refundAddress, ":")
			if len(split) != 2 {
				return params, ErrIncorrectRefundQuery
			}
			if !common.IsHexAddress(split[0]) {
				return params, ErrIncorrectRefundQuery
			}
			address := common.HexToAddress(split[0])
			percent, err := strconv.Atoi(split[1])
			if err != nil {
				return params, ErrIncorrectRefundQuery
			}
			if percent <= 0 || percent >= 100 {
				return params, ErrIncorrectRefundQuery
			}
			addresses[i] = address
			percents[i] = percent
		}

		totalRefund := 0
		for _, percent := range percents {
			totalRefund += percent
		}
		if totalRefund <= 0 || totalRefund >= 100 {
			return params, ErrIncorrectRefundQuery
		}

		// normalize refund config percentages
		for i, percent := range percents {
			percents[i] = (percent * 100) / totalRefund
		}

		// should sum to 100
		totalRefundConfDelta := 0
		for _, percent := range percents {
			totalRefundConfDelta += percent
		}
		totalRefundConfDelta = 100 - totalRefundConfDelta

		// try to remove delta
		for i, percent := range percents {
			if fixed := totalRefundConfDelta + percent; fixed <= 100 && fixed >= 0 {
				percents[i] = fixed
				break
			}
		}

		refundConfig := make([]types.RefundConfig, len(percents))
		for i, percent := range percents {
			refundConfig[i] = types.RefundConfig{
				Address: addresses[i],
				Percent: percent,
			}
		}

		params.pref.RefundConfig = refundConfig
		params.pref.WantRefund = &totalRefund
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
