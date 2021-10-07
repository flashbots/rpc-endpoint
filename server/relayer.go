package server

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
)

var methodsWithProtection = map[string]bool{
	"a9059cbb": true, // transfer
	"23b872dd": true, // transferFrom
	"095ea7b3": true, // approve
	"2e1a7d4d": true, // weth withdraw
	"d0e30db0": true, // weth deposit
	"f242432a": true, // safe transfer NFT
}

// Check if a request needs frontrunning protection. There are many transactions that don't need frontrunning protection,
// for example simple ERC20 transfers.
func EvaluateTransactionForFrontrunningProtection(reqId string, rawJsonReq *JsonRpcRequest) (bool, error) {
	// Sanity check JSON RPC parameters
	if rawJsonReq.Method == "eth_sendRawTransaction" {
		return false, nil
	}

	if len(rawJsonReq.Params) == 0 {
		return false, errors.New("invalid params")
	}

	rawTxHex, ok := rawJsonReq.Params[0].(string)
	if !ok || len(rawTxHex) < 2 {
		return false, errors.New("invalid raw transaction (wrong length)")
	}

	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return false, fmt.Errorf("invalid raw transaction: %s", err)
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		fmt.Println("error unmarshalling")
		return false, fmt.Errorf("error unmarshalling: %s", err)
	}

	ReqLog(reqId, "Evaluating transaction with hash: %v", tx.Hash())

	gas := tx.Gas()
	ReqLog(reqId, "gas: %v", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false, nil
	}

	data := hex.EncodeToString(tx.Data())
	ReqLog(reqId, "data: %v", data)

	if len(data) == 0 {
		ReqLog(reqId, "Data had a length of 0, but a gas greater than 21000. Sending cancellation tx to mempool.")
		return false, nil
	}

	needsProtection := methodsWithProtection[data[0:8]]
	return needsProtection, nil
}
