On sending a transaction, MetaMask keeps retrying until it's successful. This can mean a lot of requests for the backend,
but also can get MetaMask stuck in "pending" until the backend produces a workaround (eg. wrong nonce for `eth_getTransactionCount`).

### MM2 fix:

* on `sendRawTransaction`: memorize time the tx was received
* on `eth_getTransactionReceipt`: if result is `null` and tx submission time is >16 min:
  * call `eth_getBundleStatusByTransactionHash` on BE
  * if `result.status` is `FAILED_BUNDLE` then blacklist the EOA so subsequent calls to `eth_getTransactionCount` return a high nonce

Note: 16 minutes is chosen as the time interval because MetaMask increases the time between retries every block with a limit of 15 minutes. If MetaMask has not sent a retry in 15 minutes and the BE has a status of failed for the tx we can confidently conclude that the tx has been dropped.
