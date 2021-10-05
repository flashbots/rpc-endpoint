package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/ethereum/go-ethereum/core/types"
)

var allowedFunctions = []string{
	"a9059cbb", // transfer
	"23b872dd", // transferFrom
	"095ea7b3", // approve
	"2e1a7d4d", // weth withdraw
	"d0e30db0", // weth deposit
	"f242432a", // safe transfer NFT
}

var ofacBlacklist = []string{
	// OFAC banned addresses
	"0x8576acc5c05d6ce88f4e49bf65bdf0c62f91353c",
	"0xd882cFc20F52f2599D84b8e8D58C7FB62cfE344b",
	"0x901bb9583b24D97e995513C6778dc6888AB6870e",
	"0xa7e5d5a720f06526557c513402f2e6b5fa20b00",  // this is an invalid address, but is what"s listed in the ofac ban list
	"0xA7e5d5A720f06526557c513402f2e6B5fA20b008", // the actual valid address
	"0x7F367cC41522cE07553e823bf3be79A889DEbe1B",
	"0x1da5821544e25c636c1417Ba96Ade4Cf6D2f9B5A",
	"0x7Db418b5D567A4e0E8c59Ad71BE1FcE48f3E6107",
	"0x72a5843cc08275C8171E582972Aa4fDa8C397B2A",
	"0x7F19720A857F834887FC9A7bC0a0fBe7Fc7f8102",
	"0x9F4cda013E354b8fC285BF4b9A60460cEe7f7Ea9",
}

type PrivateTxRelayer struct {
	TxManagerUrl string
	id           uint64
}

func NewPrivateTxRelayer() *PrivateTxRelayer {
	relayer := &PrivateTxRelayer{
		TxManagerUrl: "https://protection.flashbots.net/v1/rpc",
		id:           uint64(1e9),
	}

	return relayer
}

func (r *PrivateTxRelayer) _sendTransaction(rawJsonReq *JsonRpcRequest, url string) (*JsonRpcResponse, error) {
	// Validate JSON RPC parameters:
	if len(rawJsonReq.Params) == 0 {
		return nil, errors.New("invalid params")
	}

	// Prepare eth_sendRawTransaction JSON-RPC request
	rawTxHex, ok := rawJsonReq.Params[0].(string)
	fmt.Printf("Raw tx: %s", rawTxHex)
	if !ok || len(rawTxHex) < 2 {
		return nil, errors.New("invalid raw transaction")
	}
	jsonData, err := json.Marshal(JsonRpcMessage{
		ID:      1,
		Version: "2.0",
		Method:  "eth_sendRawTransaction",
		Params:  []string{rawTxHex},
	})
	if err != nil {
		return nil, err
	}

	// Execute eth_sendRawTransaction JSON-RPC request
	log.Printf("Json data: %s", jsonData)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending tx (sending request): %s", err)
		return nil, err
	}

	// Check response for errors
	log.Printf("resp: %v", resp)
	respData, err := ioutil.ReadAll(resp.Body)
	log.Printf("respData: %s", respData)
	if err != nil {
		log.Printf("Error sending tx (reading body): %s", err)
		return nil, err
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(JsonRpcResponse)
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		log.Printf("Error sending tx (decoding json rpc response): %s", err)
		return nil, err
	}

	if jsonRpcResp.Error != nil {
		return nil, *jsonRpcResp.Error
	}

	// Prepare JSON-RPC response with result
	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return nil, errors.New("invalid raw transaction")
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		fmt.Println("error unmarshalling")
		return nil, errors.New("error unmarshalling")
	}

	jsonResp := &JsonRpcResponse{
		Id:      rawJsonReq.Id,
		Result:  tx.Hash().Hex(),
		Version: "2.0",
	}

	return jsonResp, nil
}

func (r *PrivateTxRelayer) SendTransactionToMempool(rawJsonReq *JsonRpcRequest, url string) (*JsonRpcResponse, error) {
	return r._sendTransaction(rawJsonReq, url)
}

// TxManagers manage the submission of transactions. They repeatedly submit transactions as bundles and monitor for inclusion.
// Currently the Flashbots team operates one which you can post eth_sendRawTransaction json rpc calls to.
// We post proxied transactions to the txManager
func (r *PrivateTxRelayer) SendToTxManager(rawJsonReq *JsonRpcRequest) (*JsonRpcResponse, error) {
	return r._sendTransaction(rawJsonReq, r.TxManagerUrl)
}

func (r *PrivateTxRelayer) checkForOFACList(rawJsonReq *JsonRpcRequest) (bool, error) {
	// Validate JSON RPC parameters:
	if len(rawJsonReq.Params) == 0 {
		return false, errors.New("invalid params")
	}
	rawTxHex, ok := rawJsonReq.Params[0].(string)
	// fmt.Printf("Raw tx: %s", rawTxHex)
	if !ok || len(rawTxHex) < 2 {
		return false, errors.New("invalid raw transaction")
	}
	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return false, errors.New("invalid raw transaction")
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		fmt.Println("error unmarshalling")
		return false, errors.New("error unmarshalling")
	}

	from, err := From(tx)
	if err != nil {
		return false, errors.New("error getting from")
	}

	log.Printf("from: %v", from)

	for _, address := range ofacBlacklist {
		if from == address {
			return true, nil
		}
	}
	return false, nil
}

func (r *PrivateTxRelayer) EvaluateTransactionForFrontrunningProtection(rawJsonReq *JsonRpcRequest) (bool, error) {
	// Validate JSON RPC parameters:
	if len(rawJsonReq.Params) == 0 {
		return false, errors.New("invalid params")
	}
	rawTxHex, ok := rawJsonReq.Params[0].(string)
	// fmt.Printf("Raw tx: %s", rawTxHex)
	if !ok || len(rawTxHex) < 2 {
		return false, errors.New("invalid raw transaction")
	}
	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return false, errors.New("invalid raw transaction")
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		fmt.Println("error unmarshalling")
		return false, errors.New("error unmarshalling")
	}
	log.Printf("Evaluating transaction with hash: %v", tx.Hash())

	gas := tx.Gas()
	log.Printf("gas: %v", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false, nil
	}

	// There are many transactions that don't need frontrunning protection, for example simple ERC20 transfers
	// In checkForFunctions() we check to see if the function selector being called is one that we know doesn't need frontrunning protection
	// If checkForFunctions() returns false then that means this transaction does not need frontrunning protection
	data := hex.EncodeToString(tx.Data())
	log.Printf("data: %v", data)

	if len(data) == 0 {
		log.Printf("Data had a length of 0, but a gas greater than 21000. Sending cancellation tx to mempool.")
		return false, nil
	}

	protect := checkForFunctions(data[0:8])
	if !protect {
		return false, nil
	}

	return true, nil
}

func From(tx *types.Transaction) (string, error) {
	signer := types.LatestSignerForChainID(tx.ChainId())
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return "", err
	}
	return sender.Hex(), nil
}

func checkForFunctions(data string) bool {
	for _, selector := range allowedFunctions {
		if data == selector {
			return false
		}
	}

	return true
}
