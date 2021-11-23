package server

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/pkg/errors"
)

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
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
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

func SendRpcWithSignatureAndParseResponse(url string, privKey *ecdsa.PrivateKey, jsonRpcReq *types.JsonRpcRequest) (jsonRpcResponse *types.JsonRpcResponse, responseBytes *[]byte, err error) {
	body, err := json.Marshal(jsonRpcReq)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal")
	}

	// fmt.Printf("body: %s\n", body)
	hashedBody := crypto.Keccak256Hash([]byte(body)).Hex()
	sig, err := crypto.Sign(accounts.TextHash([]byte(hashedBody)), privKey)
	if err != nil {
		return nil, nil, err
	}
	signature := crypto.PubkeyToAddress(privKey.PublicKey).Hex() + ":" + hexutil.Encode(sig)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Flashbots-Signature", signature)
	httpClient := &http.Client{
		Timeout: time.Second * 30,
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "post")
	}
	defer response.Body.Close()

	respData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "read")
	}

	jsonRpcResp := new(types.JsonRpcResponse)
	errorResp := new(types.RelayErrorResponse)
	if err := json.Unmarshal(respData, errorResp); err == nil && errorResp.Error != "" {
		// relay returned an error. Convert to standard JSON-RPC error
		jsonRpcResp.Error = &types.JsonRpcError{Message: errorResp.Error}
		return jsonRpcResp, &respData, nil
	}

	// Unmarshall JSON-RPC response and check for error inside
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		// fmt.Printf("unmarshal error. data: %s\n", respData)
		return nil, &respData, errors.Wrap(err, "unmarshal")
	}

	return jsonRpcResp, &respData, nil
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
		return nil, errors.Wrap(err, "privTxApi jsonUnmarshal failed for "+txHash)
	}

	return respObj, nil
}
