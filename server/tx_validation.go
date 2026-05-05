package server

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
)

// EIP-7825 transaction gas limit cap.
// https://eips.ethereum.org/EIPS/eip-7825
//
// Defined locally rather than imported from go-ethereum's params.MaxTxGas
// because the pinned go-ethereum version (v1.15.2) predates EIP-7825.
// Switch to params.MaxTxGas the next time go-ethereum is bumped to >= v1.16.5.
const maxTxGasEIP7825 uint64 = 1 << 24 // 16,777,216

var errGasLimitTooHigh = errors.New("transaction gas limit too high")

// validateTxLimits runs RPC-layer validation that should reject a tx before it
// is forwarded to the relay. Returns a non-nil error if the tx must be rejected.
//
// Currently checks:
//   - EIP-7825: tx.Gas() must be <= maxTxGasEIP7825 (16,777,216)
func validateTxLimits(tx *types.Transaction) error {
	if tx == nil {
		return errors.New("nil transaction")
	}
	if tx.Gas() > maxTxGasEIP7825 {
		return fmt.Errorf("%w: tx gas limit %d exceeds EIP-7825 cap %d",
			errGasLimitTooHigh, tx.Gas(), maxTxGasEIP7825)
	}
	return nil
}
