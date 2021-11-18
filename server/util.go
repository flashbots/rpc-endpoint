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
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

// CHROME_ID: nkbihfbeogaeaoehlefnkodbefgpgknn
func IsMetamask(r *http.Request) bool {
	return r.Header.Get("Origin") == "chrome-extension://nkbihfbeogaeaoehlefnkodbefgpgknn"
}

// FIREFOX_ID: webextension@metamask.io
func IsMetamaskMoz(r *http.Request) bool {
	return r.Header.Get("Origin") == "moz-extension://57f9aaf6-270a-154f-9a8a-632d0db4128c"
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

func SendRpcAndParseResponseTo(url string, req *JsonRpcRequest) (*JsonRpcResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "marshal")
	}

	// fmt.Printf("%s\n", jsonData)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "post")
	}

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read")
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(JsonRpcResponse)
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	return jsonRpcResp, nil
}

type RelayErrorResponse struct {
	Error string `json:"error"`
}

func SendRpcWithSignatureAndParseResponse(url string, privKey *ecdsa.PrivateKey, jsonRpcReq *JsonRpcRequest) (jsonRpcResponse *JsonRpcResponse, responseBytes *[]byte, err error) {
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

	jsonRpcResp := new(JsonRpcResponse)
	errorResp := new(RelayErrorResponse)
	if err := json.Unmarshal(respData, errorResp); err == nil && errorResp.Error != "" {
		// relay returned an error. Convert to standard JSON-RPC error
		jsonRpcResp.Error = &JsonRpcError{Message: errorResp.Error}
		return jsonRpcResp, &respData, nil
	}

	// Unmarshall JSON-RPC response and check for error inside
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		// fmt.Printf("unmarshal error. data: %s\n", respData)
		return nil, &respData, errors.Wrap(err, "unmarshal")
	}

	return jsonRpcResp, &respData, nil
}

func GetTxStatus(txHash string) (*PrivateTxApiResponse, error) {
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

	respObj := new(PrivateTxApiResponse)
	err = json.Unmarshal(bodyBytes, respObj)
	if err != nil {
		return nil, errors.Wrap(err, "privTxApi jsonUnmarshal failed for "+txHash)
	}

	State.txStatus[strings.ToLower(txHash)] = NewStringWithTime(respObj.Status)
	return respObj, nil
}

func ShouldSendTxToRelay(txHash string) bool {
	// send again if tx is failed
	txStatus, ok := State.txStatus[strings.ToLower(txHash)]
	if ok && txStatus.s == "FAILED" {
		return true
	}

	// don't send again tx again for 20 minutes (unless it's failed)
	txSentToRelayAt, ok := State.txForwardedToRelay[strings.ToLower(txHash)]
	if ok && time.Since(txSentToRelayAt).Minutes() < 20 {
		return false
	}

	return true
}
