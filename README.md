## Flashbots RPC Endpoint (_flashbots-rpc-endpoint_)

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)
![Discord](https://img.shields.io/discord/755466764501909692)

This repository contains code for a simple server which can be used as an RPC endpoint in popular Ethereum wallets.

The endpoint is **https://rpc.flashbots.net/**

It does two basic things:
- First, it receives json-rpc requests, proxies those to a node, and responds with the result of the proxied request.
- Second, it sends transactions to a "transaction manager," which manages the submission of that transaction to Flashbots. Currently that transaction manager is the Flashbots Protect API by default.

## Usage

To send your transactions through the Flashbots Protect RPC please refer to the [quick-start guide](https://docs.flashbots.net/flashbots-protect/rpc/quick-start/).

To run the server, run the following command:

```bash
go run main.go --listen 127.0.0.1:9000 --proxy PROXY_URL
```

Example call:

```bash
curl localhost:9000 -f -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", false],"id":1}'
```

## Transaction frontrunning protection evaluation rules

Not all transactions need frontrunning protection, and in fact some transactions cannot be sent to Flashbots at all. To reflect this we evaluate transactions in two ways:
- Does the transaction use more than 42,000 gas? If it doesn't then the Flashbots Relay will reject it, and we're not aware of use cases that use such low gas that need frontrunning protection. Thus, we send low gas transactions to the mempool.
- Does the transaction call one of a few whitelisted functions, such as an ERC20 approval, that don't need frontrunning protection? If so then we send it to the mempool.

We're open to new ways of evaluating what needs frontrunning protection and welcome PRs to this end.

## Maintainers

This project is currently maintained by:

* [@bertcmiller](https://twitter.com/bertcmiller)
* [@metachris](https://twitter.com/metachris)

## Contributing

[Flashbots](https://flashbots.net) is a research and development collective working on mitigating the negative externalities of decentralized economies. We contribute with the larger free software community to illuminate the dark forest.

You are welcome here <3.

- If you want to join us, come and say hi in our [Discord chat](https://discord.gg/7hvTycdNcK).
- If you have a question, feedback or a bug report for this project, please [open a new Issue](https://github.com/flashbots/rpc-endpoint/issues).
- If you would like to contribute with code, check the [CONTRIBUTING file](CONTRIBUTING.md).
- We just ask you to be nice.

## Security

If you find a security vulnerability on this project or any other initiative related to Flashbots, please let us know sending an email to security@flashbots.net.

## License

The code in this project is free software under the [MIT license](LICENSE).

---

Made with ☀️  by the ⚡🤖 collective.