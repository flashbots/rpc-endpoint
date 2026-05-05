package server

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// validateTxLimits runs RPC-layer validation that should reject a tx before it
// is forwarded to the relay. Returns a non-nil error if the tx must be rejected.
//
// Currently checks:
//   - EIP-7825: tx.Gas() must be <= params.MaxTxGas (16,777,216)
func validateTxLimits(tx *types.Transaction) error {
	if tx == nil {
		return errors.New("nil transaction")
	}
	if tx.Gas() > params.MaxTxGas {
		return fmt.Errorf("%w: tx gas limit %d exceeds EIP-7825 cap %d",
			core.ErrGasLimitTooHigh, tx.Gas(), params.MaxTxGas)
	}
	return nil
}
