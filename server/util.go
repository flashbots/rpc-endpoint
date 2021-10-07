package server

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
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

func ReqLog(requestId string, format string, v ...interface{}) {
	prefix := fmt.Sprintf("[%s] ", requestId)
	log.Printf(prefix+format, v...)
}

func TxFrom(tx *types.Transaction) (string, error) {
	signer := types.LatestSignerForChainID(tx.ChainId())
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return "", err
	}
	return sender.Hex(), nil
}

func RawTxFrom(rawTxHex string) (string, error) {
	if len(rawTxHex) < 2 {
		return "", errors.New("invalid raw transaction")
	}

	rawTxBytes, err := hex.DecodeString(rawTxHex[2:])
	if err != nil {
		return "", errors.New("invalid raw transaction")
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		fmt.Println("error unmarshalling:", err)
		return "", errors.New("error unmarshalling")
	}

	from, err := TxFrom(tx)
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
