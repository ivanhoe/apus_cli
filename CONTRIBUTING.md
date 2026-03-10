# Contributing

This repo contains the Go CLI for integrating [Apus](https://github.com/ivanhoe/apus) into iOS projects.

## Local setup

You will usually want:

- Go
- Xcode 15+
- Xcode command line tools
- at least one iPhone simulator runtime
- `xcodegen`
- a local checkout of `apus` when running the heavier fixture and release smoke flows

## Build and test

```bash
go build -o apus .
go run . --help
go test ./...
go vet ./...
```

## Documentation split

- [`README.md`](./README.md): public user-facing product docs
- [`docs/support-matrix.md`](./docs/support-matrix.md): what is supported, risky, and unsupported
- [`docs/troubleshooting.md`](./docs/troubleshooting.md): user-facing failure recovery
- [`docs/mcp-clients.md`](./docs/mcp-clients.md): connecting MCP clients
- [`AGENTS.md`](./AGENTS.md): repo-specific guidance for coding agents and working memory

## Fixture matrix

The fixture matrix is the source of truth for end-to-end validation coverage:

- [`fixtures/matrix.json`](./fixtures/matrix.json)
- [`fixtures/README.md`](./fixtures/README.md)

Useful commands:

```bash
go run ./tools/fixturematrix validate
go run ./tools/fixturematrix list
go run ./tools/fixturematrix plan
go build -o .tmp/bin/apus .
go build -o .tmp/bin/fixturerunner ./tools/fixturerunner
./.tmp/bin/fixturerunner -apus-bin ./.tmp/bin/apus -apus-package-path ../apus
```

The external fixtures in the matrix are pinned by SHA and cloned into a temp workspace by `fixturerunner`.

## Release smoke

Before publishing a release, validate the packaged binary:

```bash
go build -o .tmp/bin/apus .
APUS_PACKAGE_PATH=../apus ./scripts/release-smoke.sh ./.tmp/bin/apus
```

The release workflow also runs packaged-binary smoke validation from GitHub Actions.

## Contribution expectations

- every regression should become a test or fixture
- if a bug is found on a real repo shape, add either a synthetic regression or a pinned external fixture
- keep command behavior explicit: prefer fail-fast over silent heuristics
- when touching public behavior, update the public docs in `README.md` or `docs/`

## Before opening a PR

At minimum:

```bash
go test ./...
go vet ./...
```

When touching command contracts, integration flows, or release behavior, also run:

```bash
go run ./tools/fixturematrix validate
go run ./tools/fixturematrix plan
./.tmp/bin/fixturerunner -apus-bin ./.tmp/bin/apus -apus-package-path ../apus
APUS_PACKAGE_PATH=../apus ./scripts/release-smoke.sh ./.tmp/bin/apus
```
