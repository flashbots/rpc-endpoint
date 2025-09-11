package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigurationWatcherPresets(t *testing.T) {
	// Test the core business logic: valid presets are parsed and available
	config := CustomersConfig{
		URLs: map[string][]string{
			"quicknode": {"/fast?originId=quicknode"},
		},
		Presets: map[string]string{
			"quicknode": "/fast?originId=quicknode&refund=0x1234567890123456789012345678901234567890:90",
		},
	}

	watcher, err := NewConfigurationWatcher(config)
	require.NoError(t, err)
	require.NotNil(t, watcher)

	// Core functionality: preset should be parsed and available
	preset, exists := watcher.ParsedPresets["quicknode"]
	require.True(t, exists)
	require.Equal(t, "quicknode", preset.originId)
	require.True(t, preset.fast)
	require.Equal(t, 1, len(preset.pref.Validity.Refund))
}

func TestConfigurationWatcherInvalidPresets(t *testing.T) {
	// Test graceful degradation: invalid presets are skipped, don't break startup
	config := CustomersConfig{
		URLs: map[string][]string{
			"test": {"/fast?originId=test"},
		},
		Presets: map[string]string{
			"valid":   "/fast?originId=valid",
			"invalid": "://invalid-url", // This should be skipped
		},
	}

	watcher, err := NewConfigurationWatcher(config)
	require.NoError(t, err) // Should not fail startup

	// Valid preset loaded
	_, exists := watcher.ParsedPresets["valid"]
	require.True(t, exists)

	// Invalid preset skipped
	_, exists = watcher.ParsedPresets["invalid"]
	require.False(t, exists)
}
