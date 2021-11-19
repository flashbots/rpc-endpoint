On sending a transaction:

* MetaMask continuously checks for status with `eth_getTransactionReceipt`, blocking the UI in "tx pending" state.
* The TX never makes it to the mempool if it failed (not enough gas, reverts, etc.)
* MetaMask doesn't know about it and keeps resubmitting transactions (all ~20 sec)

On `eth_getTransactionReceipt`, we check whether the private tx failed with the private-tx-api. If the TX failed, we return a wrong nonce for `eth_getTransactionCount` to
unblock the MetaMask UI (see also https://github.com/MetaMask/metamask-extension/issues/10914).

Note: MetaMask needs to receive a wrong nonce exactly 4 times, after which it will put the transaction into "Dropped" state.

There's a helper to debug the MetaMask behaviour: set the `DEBUG_DONT_SEND_RAWTX` environment variable to `1`, and transactions won't be sent at all. The Metamask fix flow
will be triggered immediately (i.e. next `getTransactionReceipt` call starts the nonce fix without waiting for tx to expire).
