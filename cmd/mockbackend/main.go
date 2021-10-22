package main

import (
	"fmt"
	"net/http"

	"github.com/flashbots/rpc-endpoint/test"
)

func main() {
	port := 8090
	http.HandleFunc("/", test.RpcBackendHandler)
	fmt.Printf("rpc backend listening on localhost:%d\n", port)
	http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)

}
