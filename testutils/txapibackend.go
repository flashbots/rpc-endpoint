package testutils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/flashbots/rpc-endpoint/rpctypes"
)

var MockTxApiStatusForHash map[string]rpctypes.PrivateTxStatus = make(map[string]rpctypes.PrivateTxStatus)

func MockTxApiReset() {
	MockTxApiStatusForHash = make(map[string]rpctypes.PrivateTxStatus)
}

func MockTxApiHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	fmt.Println("TX API", req.URL)

	if !strings.HasPrefix(req.URL.Path, "/tx/") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	txHash := req.URL.Path[4:] // by default, the first 4 characters are "/tx/"
	resp := rpctypes.PrivateTxApiResponse{Status: rpctypes.TxStatusUnknown}

	if status, found := MockTxApiStatusForHash[txHash]; found {
		resp.Status = status
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("error writing response 2: %v - data: %v", err, resp)
	}
}
