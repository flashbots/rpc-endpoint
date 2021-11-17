On sending a transaction:

* MetaMask continuously checks for status with `eth_getTransactionReceipt`, blocking the UI in "tx pending" state.
* The TX never makes it to the mempool if it failed (not enough gas, reverts, etc.)
* MetaMask doesn't know about it and keeps resubmitting transactions (all ~20 sec)

Once we detect a failed tx internally, we release the MM UI by returning a wrong nonce for `eth_getTransactionCount` (see also
https://github.com/MetaMask/metamask-extension/issues/10914)


### MM fix2:

* on `sendRawTransaction` remember the time tx was received from user
* on `eth_getTransactionReceipt`: if submission time is >14 min and result is `null` then:
  * call `eth_getBundleStatusByTransactionHash` on BE
  * if `result.status` is `FAILED_BUNDLE` then blacklist the EOA so subsequent calls to `eth_getTransactionCount` return a high nonce

Note: 14 minutes is chosen as the time interval because MetaMask increases the time between retries every block with a limit of 15 minutes.
If MetaMask has not sent a retry in 15 minutes and the BE has a status of failed for the tx we can confidently conclude that the tx has been dropped.