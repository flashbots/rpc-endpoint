package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWhitelist(t *testing.T) {
	require.True(t, isOnFunctionWhitelist("23b872dd"))
	require.False(t, isOnFunctionWhitelist("23b872dx"))
}

func TestWhitelistLowercase(t *testing.T) {
	for v := range allowedFunctions {
		require.Equal(t, v, strings.ToLower(v))
	}

	for v := range allowedLargeTxTargets {
		require.Equal(t, v, strings.ToLower(v))
	}
}
