package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/flashbots/rpc-endpoint/server"
)

func TestFingerprint_ToIPv6(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	req.Header.Set("X-Forwarded-For", "2600:8802:4700:bee:d13c:c7fb:8e0f:84ff, 172.70.210.100")
	fingerprint1, err := server.FingerprintFromRequest(req, time.Date(2022, 1, 1, 1, 2, 3, 4, time.UTC))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(fingerprint1.ToIPv6().String(), "2001:db8::"))

	fingerprint2, err := server.FingerprintFromRequest(req, time.Date(2022, 1, 1, 2, 3, 4, 5, time.UTC))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(fingerprint2.ToIPv6().String(), "2001:db8::"))

	require.NotEqual(t, fingerprint1, fingerprint2)
}
