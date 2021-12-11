## Flashbots Protect RPC Endpoint

[![Test status](https://github.com/flashbots/rpc-endpoint/workflows/Test/badge.svg)](https://github.com/flashbots/rpc-endpoint/actions?query=workflow%3A%22Test%22)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)
[![Discord](https://img.shields.io/discord/755466764501909692)](https://discord.gg/7hvTycdNcK)

This repository contains code for a server which can be used as an RPC endpoint in popular Ethereum wallets.

The endpoint is live at **https://rpc.flashbots.net/**

It does two basic things:
- It receives JSON-RPC requests, proxies those to a node, and responds with the result of the proxied request.
- On receiving an `eth_sendRawTransaction` call with 42000 gas or more (and not on whitelisted method), the call is sent to the Flashbots relay as a private transaction, and submitted as bundles for up to 25 blocks.

There are a few key benefits to using the Flashbots RPC endpoint:

- Frontrunning protection: your transaction will not be seen by hungry sandwich bots in the public mempool.
- No failed transactions: your transaction will only be mined if it doesn't include any reverts, so you don't pay for failed transactions. Note: your transaction could be uncled, emitted to the mempool, and then included on-chain.
- Priority in blocks: transactions sent via Flashbots are mined at the top of blocks, giving them priority.

## Transaction Status Check

If a transaction is sent to the Flashbots relay instead of the public mempool, you cannot see the status on Etherscan or other explorers. Flashbots provides a Protect Transaction API to get the status of these private transactions: **https://protect.flashbots.net/**

## Transaction Frontrunning Protection Evaluation Rules

Not all transactions need frontrunning protection, and in fact some transactions cannot be sent to Flashbots at all. To reflect this we evaluate transactions in two ways:
- Does the transaction use more than 42,000 gas? If it doesn't then the Flashbots Relay will reject it, and we're not aware of use cases that use such low gas that need frontrunning protection. Thus, we send low gas transactions to the mempool.
- Does the transaction call one of a few whitelisted functions, such as an ERC20 approval, that don't need frontrunning protection? If so then we send it to the mempool.

We're open to new ways of evaluating what needs frontrunning protection and welcome PRs to this end.

## Usage

To send your transactions through the Flashbots Protect RPC please refer to the [quick-start guide](https://docs.flashbots.net/flashbots-protect/rpc/quick-start/).

To run the server, run the following command:

```bash
go run cmd/server/main.go -redis REDIS_URL -signingKey ETH_PRIVATE_KEY -proxy PROXY_URL

# For development, you can use built-in redis and create a random signing key
go run cmd/server/main.go -redis dev -signingKey dev -proxy PROXY_URL

# You can use the DEBUG_DONT_SEND_RAWTX to skip sending transactions anywhere (useful for local testing):
DEBUG_DONT_SEND_RAWTX=1 go run cmd/server/main.go -redis dev -signingKey dev -proxy PROXY_URL
```

Example call:

```bash
curl localhost:9000 -f -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", false],"id":1}'
```

## Maintainers

This project is currently maintained by:

* [@bertcmiller](https://twitter.com/bertcmiller)
* [@metachris](https://twitter.com/metachris)

## Contributing

[Flashbots](https://flashbots.net) is a research and development collective working on mitigating the negative externalities of decentralized economies. We contribute with the larger free software community to illuminate the dark forest.

You are welcome here <3.

- If you want to join us, come and say hi in our [Discord chat](https://discord.gg/7hvTycdNcK).
- If you have a question, feedback or a bug report for this project, please [open a new Issue](https://github.com/flashbots/rpc-endpoint/issues).
- We ask you to be nice.

**Send a pull request**

- Your proposed changes should be first described and discussed in an issue.
- Every pull request should be small and represent a single change. If the problem is complicated, split it in multiple issues and pull requests.
- Every pull request should be covered by unit/e2e tests.

We appreciate your contributions <3

## Security

If you find a security vulnerability on this project or any other initiative related to Flashbots, please let us know sending an email to security@flashbots.net.

## License

The code in this project is free software under the [MIT license](LICENSE).

---

Made with ☀️  by the ⚡🤖 collective.
