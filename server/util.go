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

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/pkg/errors"
)

func Min(a uint64, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func Max(a uint64, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
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

func GetTx(rawTxHex string) (*ethtypes.Transaction, error) {
	if len(rawTxHex) < 2 {
		return nil, errors.New("invalid raw transaction")
	}

	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return nil, errors.New("invalid raw transaction")
	}

	tx := new(ethtypes.Transaction)
	if err = tx.UnmarshalBinary(rawTxBytes); err != nil {
		return nil, errors.New("error unmarshalling")
	}

	return tx, nil
}

func GetSenderFromTx(tx *ethtypes.Transaction) (string, error) {
	signer := ethtypes.LatestSignerForChainID(tx.ChainId())
	sender, err := ethtypes.Sender(signer, tx)
	if err != nil {
		return "", err
	}
	return sender.Hex(), nil
}

func GetSenderFromRawTx(tx *ethtypes.Transaction) (string, error) {
	from, err := GetSenderFromTx(tx)
	if err != nil {
		return "", errors.New("error getting from")
	}

	return from, nil
}

func GetTxStatus(txHash string) (*types.PrivateTxApiResponse, error) {
	privTxApiUrl := fmt.Sprintf("%s/tx/%s", ProtectTxApiHost, txHash)
	resp, err := http.Get(privTxApiUrl)
	if err != nil {
		return nil, errors.Wrap(err, "privTxApi call failed for "+txHash)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "privTxApi body-read failed for "+txHash)
	}

	respObj := new(types.PrivateTxApiResponse)
	err = json.Unmarshal(bodyBytes, respObj)
	if err != nil {
		msg := fmt.Sprintf("privTxApi jsonUnmarshal failed for %s - status: %d / body: %s", txHash, resp.StatusCode, string(bodyBytes))
		return nil, errors.Wrap(err, msg)
	}

	return respObj, nil
}
