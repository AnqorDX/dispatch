# Contributing to dispatch

Thank you for your interest in contributing. This document explains how to get involved.

## Before You Start

All contributors must sign the [Contributor License Agreement](CLA.md) before a pull request can be merged. CLA Assistant will prompt you automatically when you open your first PR.

## Reporting Issues

Open a [GitHub Issue](https://github.com/AnqorDX/dispatch/issues) and include:

- A clear description of the problem or proposal
- A minimal reproduction case if reporting a bug
- The Go version and OS you are running

## Making Changes

1. Fork the repository and create a branch from `main`.
2. Make your changes. Keep commits focused — one logical change per commit.
3. Ensure all existing tests pass: `go test -race ./...`
4. Add tests for any new behaviour. The existing test file is `dispatch_test.go`.
5. Open a pull request against `main` with a clear description of what the change does and why.

## Code Style

- Follow standard Go conventions. Run `gofmt` before committing.
- Exported symbols must have doc comments.
- No external dependencies. `dispatch` has none and should stay that way.

## What Gets Merged

Changes that are likely to be accepted:

- Bug fixes with a failing test that demonstrates the bug
- Performance improvements with a benchmark showing the gain
- Documentation improvements that are accurate and concise

Changes that are unlikely to be accepted:

- New dependencies
- Features that broaden the scope beyond a focused event bus
- Breaking changes to the public API

When in doubt, open an issue first to discuss the proposal before writing code.

## License

By contributing, you agree that your contributions will be licensed under the [Apache 2.0 License](LICENSE).
