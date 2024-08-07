package flashbots_test

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/flashbots/rpc-endpoint/adapters/flashbots"
)

func TestParseSignature(t *testing.T) {

	// For most of these test cases, we first need to generate a signature
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	body := fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["%s","pending"],"id":1}`,
		address,
	)

	signature, err := crypto.Sign(
		accounts.TextHash([]byte(hexutil.Encode(crypto.Keccak256([]byte(body))))),
		privateKey,
	)
	require.NoError(t, err)

	header := fmt.Sprintf("%s:%s", address, hexutil.Encode(signature))

	t.Run("header is empty", func(t *testing.T) {
		_, err := flashbots.ParseSignature("", []byte{})
		require.ErrorIs(t, err, flashbots.ErrNoSignature)
	})

	t.Run("header is valid", func(t *testing.T) {
		signingAddress, err := flashbots.ParseSignature(header, []byte(body))
		require.NoError(t, err)
		require.Equal(t, address, signingAddress)
	})

	t.Run("header is invalid", func(t *testing.T) {
		_, err := flashbots.ParseSignature("invalid", []byte(body))
		require.ErrorIs(t, err, flashbots.ErrInvalidSignature)
	})

	t.Run("header has extra bytes", func(t *testing.T) {
		_, err := flashbots.ParseSignature(header+"deadbeef", []byte(body))
		require.ErrorIs(t, err, flashbots.ErrInvalidSignature)
	})

	t.Run("header has missing bytes", func(t *testing.T) {
		_, err := flashbots.ParseSignature(header[:len(header)-8], []byte(body))
		require.ErrorIs(t, err, flashbots.ErrInvalidSignature)
	})

	t.Run("body is empty", func(t *testing.T) {
		_, err := flashbots.ParseSignature(header, []byte{})
		require.ErrorIs(t, err, flashbots.ErrInvalidSignature)
	})

	t.Run("body is invalid", func(t *testing.T) {
		_, err := flashbots.ParseSignature(header, []byte(`{}`))
		require.ErrorIs(t, err, flashbots.ErrInvalidSignature)
	})

	t.Run("body has extra bytes", func(t *testing.T) {
		_, err := flashbots.ParseSignature(header, []byte(body+"..."))
		require.ErrorIs(t, err, flashbots.ErrInvalidSignature)
	})

	t.Run("body has missing bytes", func(t *testing.T) {
		_, err := flashbots.ParseSignature(header, []byte(body[:len(body)-8]))
		require.ErrorIs(t, err, flashbots.ErrInvalidSignature)
	})
}
