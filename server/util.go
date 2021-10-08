package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
)

func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func IsMetamask(r *http.Request) bool {
	return r.Header.Get("Origin") == "chrome-extension://nkbihfbeogaeaoehlefnkodbefgpgknn"
}

func ProxyRequest(proxyUrl string, body []byte) (*http.Response, error) {
	// Create new request:
	req, err := http.NewRequest("POST", proxyUrl, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	return client.Do(req)
}

func GetTx(rawTxHex string) (*types.Transaction, error) {
	if len(rawTxHex) < 2 {
		return nil, errors.New("invalid raw transaction")
	}

	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return nil, errors.New("invalid raw transaction")
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		return nil, errors.New("error unmarshalling")
	}

	return tx, nil
}

func GetSenderFromTx(tx *types.Transaction) (string, error) {
	signer := types.LatestSignerForChainID(tx.ChainId())
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return "", err
	}
	return sender.Hex(), nil
}

func GetSenderFromRawTx(tx *types.Transaction) (string, error) {
	from, err := GetSenderFromTx(tx)
	if err != nil {
		return "", errors.New("error getting from")
	}

	return from, nil
}

func TruncateText(s string, max int) string {
	if len(s) > max {
		r := 0
		for i := range s {
			r++
			if r > max {
				return s[:i]
			}
		}
	}
	return s
}

func eth_getTransactionCount(nodeUrl string, address string) (uint64, error) {
	if address == "" {
		return 0, fmt.Errorf("[eth_getTransactionCount] no address given")
	}

	jsonData, err := json.Marshal(JsonRpcRequest{
		Id:      1,
		Version: "2.0",
		Method:  "eth_getTransactionCount",
		Params:  []interface{}{address, "latest"},
	})

	if err != nil {
		return 0, errors.Wrap(err, "[eth_getTransactionCount] failed to marshal JSON RPC request")
	}

	// Execute eth_sendRawTransaction JSON-RPC request
	resp, err := http.Post(nodeUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, errors.Wrap(err, "[eth_getTransactionCount] sending request failed")
	}

	// Check response for errors
	// fmt.Printf("[eth_getTransactionCount] resp: %v\n", resp)
	respData, err := ioutil.ReadAll(resp.Body)
	// fmt.Printf("[eth_getTransactionCount] respData: %s\n", respData)
	if err != nil {
		return 0, errors.Wrap(err, "[eth_getTransactionCount] failed reading body")
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(JsonRpcResponse)
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		return 0, errors.Wrap(err, "[eth_getTransactionCount] failed decoding json rpc response")
	}

	if jsonRpcResp.Error != nil {
		return 0, errors.Wrap(jsonRpcResp.Error, "[eth_getTransactionCount] json-rpc response error")
	}

	// getTransactionCount request here
	txCntHex := fmt.Sprintf("%v", jsonRpcResp.Result)
	txCntInt, err := strconv.ParseInt(txCntHex[2:], 16, 64)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("[eth_getTransactionCount] failed converting %s to int", txCntHex))
	}

	return uint64(txCntInt), nil
}
