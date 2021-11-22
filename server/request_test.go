package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/flashbots/rpc-endpoint/testutils"
)

func setupRedis() {
	redisServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	RState, err = NewRedisState(redisServer.Addr())
	if err != nil {
		panic(err)
	}
}

func setupMockTxApi() {
	txApiServer := httptest.NewServer(http.HandlerFunc(testutils.MockTxApiHandler))
	ProtectTxApiHost = txApiServer.URL
}

func TestRequestshouldSendTxToRelay(t *testing.T) {
	setupRedis()
	setupMockTxApi()

	// request := NewRpcRequest(nil, nil, "", "", nil)

	// // should be true on redis error (not yet connected)
	// shouldSend := request.shouldSendTxToRelay("foo")
	// require.True(t, shouldSend)

}
