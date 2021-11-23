package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/rpc-endpoint/types"
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

func SendRpcAndParseResponseTo(url string, req *types.JsonRpcRequest) (*types.JsonRpcResponse, error) {
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

	jsonRpcResp := new(types.JsonRpcResponse)

	// Check if returned an error, if so then convert to standard JSON-RPC error
	errorResp := new(types.RelayErrorResponse)
	if err := json.Unmarshal(respData, errorResp); err == nil && errorResp.Error != "" {
		// relay returned an error, convert to standard JSON-RPC error now
		jsonRpcResp.Error = &types.JsonRpcError{Message: errorResp.Error}
		return jsonRpcResp, nil
	}

	// Unmarshall JSON-RPC response and check for error inside
	if err := json.Unmarshal(respData, jsonRpcResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	return jsonRpcResp, nil
}

func BigIntPtrToStr(i *big.Int) string {
	if i == nil {
		return ""
	}
	return i.String()
}

func AddressPtrToStr(a *common.Address) string {
	if a == nil {
		return ""
	}
	return a.Hex()
}
