package test

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type TxApiResponse struct {
	Status string `json:"status"`
}

func TxApiHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	fmt.Println("TX API", req.URL)

	if !strings.HasPrefix(req.URL.Path, "/tx/") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	txHash := req.URL.Path[4:] // by default, the first 4 characters are "/tx/"
	resp := TxApiResponse{Status: "UNKNOWN"}

	if txHash == TestTx_MM2_Hash {
		resp.Status = "FAILED"
	}
	
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("error writing response 2: %v - data: %s", err, resp)
	}
}
