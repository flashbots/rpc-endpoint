package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/metrics"
	"github.com/stretchr/testify/require"
)

func TestGetEffectiveParameters(t *testing.T) {
	// Core business logic test: header with preset uses preset (ignores URL)
	config := CustomersConfig{
		Presets: map[string]string{
			"quicknode": "/fast?originId=quicknode&refund=0x1234567890123456789012345678901234567890:90",
		},
	}

	watcher, err := NewConfigurationWatcher(config)
	require.NoError(t, err)

	// Request with header and different URL parameters
	req := httptest.NewRequest(http.MethodPost, "/fast?originId=user-provided&refund=0xdadB0d80178819F2319190D340ce9A924f783711:10", nil)
	req.Header.Set("X-Flashbots-Origin", "quicknode")

	w := httptest.NewRecorder()
	respw := http.ResponseWriter(w)

	handler := &RpcRequestHandler{
		respw:                &respw,
		req:                  req,
		logger:               log.New(),
		builderNames:         []string{"flashbots"},
		configurationWatcher: watcher,
	}

	params, err := handler.getEffectiveParameters()
	require.NoError(t, err)

	// Should use preset values, ignore URL
	require.Equal(t, "quicknode", params.originId)
	require.True(t, params.fast)
	require.Equal(t, 1, len(params.pref.Validity.Refund)) // Preset refund, not URL refund
	require.Equal(t, params.pref.Validity.Refund[0].Address, common.HexToAddress("0x1234567890123456789012345678901234567890"))
}

func TestGetEffectiveParametersNoHeader(t *testing.T) {
	// Fallback behavior: no header uses URL normally
	req := httptest.NewRequest(http.MethodPost, "/fast?originId=normal-user", nil)
	// No X-Flashbots-Origin-ID header

	w := httptest.NewRecorder()
	respw := http.ResponseWriter(w)

	handler := &RpcRequestHandler{
		respw:        &respw,
		req:          req,
		logger:       log.New(),
		builderNames: []string{"flashbots"},
		configurationWatcher: &ConfigurationWatcher{
			ParsedPresets: make(map[string]URLParameters),
		},
	}

	params, err := handler.getEffectiveParameters()
	require.NoError(t, err)
	require.Equal(t, "normal-user", params.originId)
	require.True(t, params.fast)
}

func TestGetEffectiveParametersHeaderNoPreset(t *testing.T) {
	// Edge case: header present but no matching preset falls back to URL
	req := httptest.NewRequest(http.MethodPost, "/fast?originId=fallback-user", nil)
	req.Header.Set("X-Flashbots-Origin", "unknown")

	w := httptest.NewRecorder()
	respw := http.ResponseWriter(w)

	handler := &RpcRequestHandler{
		respw:        &respw,
		req:          req,
		logger:       log.New(),
		builderNames: []string{"flashbots"},
		configurationWatcher: &ConfigurationWatcher{
			ParsedPresets: make(map[string]URLParameters),
		},
	}

	params, err := handler.getEffectiveParameters()
	require.NoError(t, err)
	require.Equal(t, "fallback-user", params.originId)
}

func TestRpcRequestHandler_UrlParam(t *testing.T) {
	wrec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/?url=http://mock.url", nil)

	metrics.UrlParamUsage.Set(0)

	var rw http.ResponseWriter = wrec
	rh := NewRpcRequestHandler(log.New(), &rw, req, "", 0, nil, "", nil, nil, nil, nil, nil, nil)
	rh.process()

	require.Equal(t, uint64(1), metrics.UrlParamUsage.Get())
}
