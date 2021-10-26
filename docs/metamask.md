On sending a transaction:

* MetaMask continuously checks for status with `eth_getTransactionReceipt`, blocking the UI in "tx pending" state.
* The TX never makes it to the mempool, in particular not if it failed (not enough gas, reverts, etc.)
* MetaMask keeps resubmitting transactions (all ~20 sec)

Once we detect a failed tx internally, we release the MM UI by returning a wrong nonce for `eth_getTransactionCount` (see also
https://github.com/MetaMask/metamask-extension/issues/10914)


### MM fix1:

If TxManager backend returns an error that a bundle has been submitted too many times:

* We blacklist the rawTxHex, and don't forward them anymore to the backend
* We return an invalid nonce (1e9 + 1) for the next 4 calls to `eth_getTransactionCount`

### MM fix2:

* on `sendRawTransaction` remember the time tx was received by user
* on `eth_getTransactionReceipt`: if submission time is >14 min and result is `null` then:
  * call `eth_getBundleStatusByTransactionHash` on BE
  * if `result.status` is `FAILED_BUNDLE` then blacklist the EOA so subsequent calls to `eth_getTransactionCount` return a high nonce

Note: 14 minutes is chosen as the time interval because MetaMask increases the time between retries every block with a limit of 15 minutes.
If MetaMask has not sent a retry in 15 minutes and the BE has a status of failed for the tx we can confidently conclude that the tx has been dropped.
