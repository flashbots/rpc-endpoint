## RPC Endpoint

This repository contains code for a simple server which can be used as an RPC endpoint in popular Ethereum wallets.

The endpoint is **https://rpc.flashbots.net/**

It does two basic things:
- First, it receives json-rpc requests, proxies those to a node, and responds with the result of the proxied request.
- Second, it sends transactions to a "transaction manager," which manages the submission of that transaction to Flashbots. Currently that transaction manager is the Flashbots Protect API by default.

The server is made up of a few components:
- `server.go`: the main server code where json rpc requests are validated and handled
- `relayer.go`: handles the evaluation and relaying of transactions
- `types.go`: defines a few json rpc types
- `util.go`: contains utility functions

## Transaction frontrunning protection evaluation rules

Not all transactions need frontrunning protection, and in fact some transactions cannot be sent to Flashbots at all. To reflect this we evaluate transactions in two ways:
- Does the transaction use more than 42,000 gas? If it doesn't then the Flashbots Relay will reject it, and we're not aware of use cases that use such low gas that need frontrunning protection. Thus, we send low gas transactions to the mempool.
- Does the transaction call one of a few whitelisted functions, such as an ERC20 approval, that don't need frontrunning protection? If so then we send it to the mempool.

We're open to new ways of evaluating what needs frontrunning protection and welcome PRs to this end.

## Using the RPC endpoint as an end user

Please refer to the [quick-start guide](https://docs.flashbots.net/flashbots-protect/rpc/quick-start/)

## Usage

To run the server, run the following command:

```bash
go run main.go --listen 127.0.0.1:9000 --proxy PROXY_URL
```

Example call:

```bash
curl localhost:9000 -f -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", false],"id":1}'
```

## Maintainers

This project is currently maintained by:

* [@bertcmiller](https://twitter.com/bertcmiller)
* [@metachris](https://twitter.com/metachris)
