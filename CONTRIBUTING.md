# Contributing to muken

Thank you for helping. Bug reports, fixes, docs and ideas are all welcome. For a large change, open an issue first so we can agree on the direction before you spend time on it.

## Contributor agreement

muken is released under the Apache License 2.0. The maintainer may also offer it under other terms, for example in paid products and services. For that to be possible, every contribution comes with this short agreement. By opening a pull request you accept it.

1. You wrote the contribution, or you have the right to submit it.
2. You license it to the project under the Apache License 2.0.
3. You also give Mai Nhut Tan a perpetual, worldwide, non-exclusive, free and irrevocable license to use, change, sublicense and distribute your contribution under any terms, including commercial and closed-source terms.
4. You keep the copyright of your contribution.

If you cannot agree, please open an issue instead of a pull request. We are glad to talk about the idea.

This text is plain and short. It has not been reviewed by a lawyer.

## Before you open a pull request

- Run `go vet ./...` and `go test ./...`. Tests must not need a server, Docker or the network.
- Format with `gofmt`.
- One idea per pull request. Use Conventional Commits for the messages: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`.
