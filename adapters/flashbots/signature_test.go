package flashbots_test

import (
	"fmt"
	"strings"
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

func TestVerifySignatureFromMetaMask(t *testing.T) {
	// Source: use the "Sign Message" feature in Etherscan
	// to sign the keccak256 hash of `Hello`
	// Published to https://etherscan.io/verifySig/255560
	messageHash := crypto.Keccak256Hash([]byte("Hello")).Hex()
	require.Equal(t, `0x06b3dfaec148fb1bb2b066f10ec285e7c9bf402ab32aa78a5d38e34566810cd2`, messageHash)
	address := `0x4bE0Cd2553356b4ABb8b6a1882325dAbC8d3013D`
	signatureHash := `0xbf36915334f8fa93894cd54d491c31a89dbf917e9a4402b2779b73d21ecf46e36ff07db2bef6d10e92c99a02c1c5ea700b0b674dfa5d3ce9220822a7ebcc17101b`
	header := address + ":" + signatureHash
	signingAddress, err := flashbots.ParseSignature(
		header,
		[]byte(`Hello`),
	)
	require.NoError(t, err)
	require.Equal(t, strings.ToLower(address), strings.ToLower(signingAddress))
}

func TestVerifySignatureCast(t *testing.T) {
	// Source: use `cast wallet sign` in the `cast` CLI
	// to sign the keccak256 hash of `Hello`:
	// `cast wallet sign --interactive $(cast from-utf8 $(cast keccak Hello))`
	// NOTE: The call to from-utf8 is required as cast wallet sign
	// interprets inputs with a leading 0x as a byte array, not a string.
	// Published to https://etherscan.io/verifySig/255562
	messageHash := crypto.Keccak256Hash([]byte("Hello")).Hex()
	require.Equal(t, `0x06b3dfaec148fb1bb2b066f10ec285e7c9bf402ab32aa78a5d38e34566810cd2`, messageHash)
	address := `0x2485Aaa7C5453e04658378358f5E028150Dc7606`
	signatureHash := `0xff2aa92eb8d8c2ca04f1755a4ddbff4bda6a5c9cefc8b706d5d8a21d3aa6fe7a20d3ec062fb5a4c1656fd2c14a8b33ca378b830d9b6168589bfee658e83745cc1b`
	header := address + ":" + signatureHash
	signingAddress, err := flashbots.ParseSignature(
		header,
		[]byte(`Hello`),
	)
	require.NoError(t, err)
	require.Equal(t, strings.ToLower(address), strings.ToLower(signingAddress))
}
