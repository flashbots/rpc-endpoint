package server

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateOutboundRPCURL(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateOutboundRPCURL("https://example.com"))
	require.NoError(t, ValidateOutboundRPCURL("http://1.1.1.1:8545"))

	require.Error(t, ValidateOutboundRPCURL("http://127.0.0.1/"))
	require.Error(t, ValidateOutboundRPCURL("http://169.254.169.254/latest/meta-data/"))
	require.Error(t, ValidateOutboundRPCURL("http://10.0.0.5"))
	require.Error(t, ValidateOutboundRPCURL("http://192.168.1.1"))
	require.Error(t, ValidateOutboundRPCURL("http://172.16.0.1"))
	require.Error(t, ValidateOutboundRPCURL("http://100.64.0.1"))
	require.Error(t, ValidateOutboundRPCURL("http://localhost:8545"))
	require.Error(t, ValidateOutboundRPCURL("file:///etc/passwd"))
	require.Error(t, ValidateOutboundRPCURL("ftp://example.com"))
	require.Error(t, ValidateOutboundRPCURL("not a url"))
}

func TestValidateRedirectURL(t *testing.T) {
	t.Parallel()

	req := &http.Request{URL: mustURL(t, "http://127.0.0.1/")}
	require.Error(t, validateRedirectURL(req, nil))

	req = &http.Request{URL: mustURL(t, "https://example.com/rpc")}
	require.NoError(t, validateRedirectURL(req, nil))
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
