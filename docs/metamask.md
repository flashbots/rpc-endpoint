MetaMask tries to detect if a tx failed by looking it up onchain. Flashbots Protect reverts don't land on chain,
but MetaMask keeps looking for the tx and interface is stuck in "pending"

MetaMask also keeps resending the transaction, which results in a lot of requests for the backend.

The workaround is returning a wrong nonce for `eth_getTransactionCount`. See also
https://github.com/MetaMask/metamask-extension/issues/10914

### MM fix1:

* If out TxManager backend returns an error that a bundle has been submitted too many times, we blacklist the rawTxHex and don't forward them anymore.
* We also return an invalid nonce (1e9 + 1), for the next 4 calls to `eth_getTransactionCount`

### MM fix2:

* on `sendRawTransaction`: memorize time the tx was received
* on `eth_getTransactionReceipt`: if result is `null` and tx submission time is >14 min:
  * call `eth_getBundleStatusByTransactionHash` on BE
  * if `result.status` is `FAILED_BUNDLE` then blacklist the EOA so subsequent calls to `eth_getTransactionCount` return a high nonce

Note: 14 minutes is chosen as the time interval because MetaMask increases the time between retries every block with a limit of 15 minutes.
If MetaMask has not sent a retry in 15 minutes and the BE has a status of failed for the tx we can confidently conclude that the tx has been dropped.
