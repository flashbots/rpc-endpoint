package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWhitelistLowercase(t *testing.T) {
	for v := range allowedFunctions {
		require.Equal(t, v, strings.ToLower(v))
	}

	for v := range allowedLargeTxTargets {
		require.Equal(t, v, strings.ToLower(v))
	}
}
