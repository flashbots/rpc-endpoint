# Contributing guide

Welcome to the Flashbots collective! We just ask you to be nice when you play with us.

The server is made up of a few components:
- `server.go`: the main server code where json rpc requests are validated and handled
- `relayer.go`: handles the evaluation and relaying of transactions
- `types.go`: defines a few json rpc types
- `util.go`: contains utility functions

## Send a pull request

- Your proposed changes should be first described and discussed in an issue.
- Open the branch in a personal fork, not in the team repository.
- Every pull request should be small and represent a single change. If the problem is complicated, split it in multiple issues and pull requests.
- Every pull request should be covered by unit tests.

We appreciate you, friend <3.
