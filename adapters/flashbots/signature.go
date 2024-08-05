// Package flashbots provides methods for parsing the X-Flashbots-Signature header.
package flashbots

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrNoSignature      = errors.New("no signature provided")
	ErrInvalidSignature = errors.New("invalid signature provided")
)

func ParseSignature(header string, body []byte) (string, error) {
	if header == "" {
		return "", ErrNoSignature
	}

	splitSig := strings.Split(header, ":")
	if len(splitSig) != 2 {
		return "", ErrInvalidSignature
	}

	return VerifySignature(body, splitSig[0], splitSig[1])
}

func VerifySignature(body []byte, signingAddressStr, signatureStr string) (string, error) {
	signature, err := hexutil.Decode(signatureStr)
	if err != nil || len(signature) == 0 {
		return "", fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	if signature[len(signature)-1] >= 27 {
		signature[len(signature)-1] -= 27
	}

	hashedBody := crypto.Keccak256Hash(body).Hex()
	messageHash := accounts.TextHash([]byte(hashedBody))
	signaturePublicKeyBytes, err := crypto.Ecrecover(messageHash, signature)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	publicKey, err := crypto.UnmarshalPubkey(signaturePublicKeyBytes)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}
	signaturePubkey := *publicKey
	signaturePubKeyAddress := crypto.PubkeyToAddress(signaturePubkey).Hex()

	// case-insensitive equality check
	if !strings.EqualFold(signaturePubKeyAddress, signingAddressStr) {
		return "", fmt.Errorf("%w: signing address mismatch", ErrInvalidSignature)
	}

	signatureNoRecoverID := signature[:len(signature)-1] // remove recovery id
	if !crypto.VerifySignature(signaturePublicKeyBytes, messageHash, signatureNoRecoverID) {
		return "", fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	return signaturePubKeyAddress, nil
}
