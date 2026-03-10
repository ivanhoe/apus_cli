# apus

`apus` integrates [Apus](https://github.com/ivanhoe/apus) into iOS apps so AI agents can talk to a live MCP server running inside your simulator app.

Use it when you want a repeatable path from Xcode project to:

- runtime logs
- screenshots
- view hierarchy inspection
- UI interaction
- network history
- hot reload

## What `apus` does

- `apus doctor` checks whether your machine and project are safe to use with `apus`
- `apus new` scaffolds a SwiftUI app, builds it, launches it, and waits for MCP
- `apus init` best-effort integrates Apus into an existing Xcode project
- `apus status` shows whether Apus is already integrated
- `apus remove` reverses what `apus init` added

## Requirements

- macOS
- Xcode 15+ with command line tools
- at least one iPhone simulator runtime installed
- `xcodegen` in `PATH` for `apus new`
- an `.xcodeproj` that `apus` can detect safely

If your repo has multiple app targets, plan to pass `--target`.

## Install

```bash
# Installer script
curl -fsSL https://raw.githubusercontent.com/ivanhoe/apus_cli/main/scripts/install.sh | bash

# Or build/install from source
go install github.com/ivanhoe/apus_cli@latest
```

After install:

```bash
apus --help
apus doctor
```

## Quickstart: Existing Project

```bash
# 1) Inspect the repo first
apus doctor --path /path/to/project

# 2) If the repo has multiple app targets, choose one explicitly
apus doctor --path /path/to/project --target MyApp

# 3) Preview the exact mutations before writing
apus init --path /path/to/project --target MyApp --dry-run

# 4) Apply the integration
apus init --path /path/to/project --target MyApp

# 5) Confirm what is integrated
apus status --path /path/to/project --target MyApp
```

What `apus init` will try to do:

1. modify `project.pbxproj` to add the Apus Swift Package dependency
2. resolve Swift Package dependencies when Apus is new to the project
3. inject `import Apus` and `Apus.shared.start()` when it can detect the app entry point
4. create `AGENTS.md` if the project does not already have one
5. create backups of touched files before mutating

## Quickstart: New Project

```bash
# 1) Verify your toolchain
apus doctor

# 2) Scaffold + build + launch a new SwiftUI app
apus new MyApp

# Optional: use a custom MCP port
apus new MyApp --port 9999
```

`apus new` currently supports the `swiftui` template only.

When it succeeds, `apus` prints the MCP URL to connect to, usually:

```text
http://localhost:9847/mcp
```

## `doctor` classifications

`apus doctor` reports one of these classifications:

- `supported`: safe happy path for `apus`
- `risky`: the project may still work, but expect a manual step such as adding `Apus.shared.start()` yourself
- `unsupported`: `apus` should stop before mutating files

Recommended workflow:

```bash
apus doctor --path /path/to/project --target MyApp
apus init --path /path/to/project --target MyApp --dry-run
apus init --path /path/to/project --target MyApp
```

## Command Guide

### `apus doctor`

Checks your machine and, optionally, a specific project.

```bash
apus doctor
apus doctor --path /path/to/project
apus doctor --path /path/to/project --target MyApp
apus doctor --json --path /path/to/project
```

### `apus new <AppName>`

Creates a new SwiftUI app with Apus pre-integrated, builds it, launches it, and waits for MCP.

```bash
apus new MyApp
apus new MyApp --port 9999   # custom MCP port for generated app + health check
```

### `apus init`

Best-effort integration for an existing Xcode project.

```bash
apus init --path /path/to/project
apus init --path /path/to/project --target MyApp
apus init --path /path/to/project --dry-run
apus init --path /path/to/project --dry-run --json
apus init --path /path/to/project --package-path /path/to/local/apus
```

### `apus status`

Reports whether Apus is integrated and which components are present.

```bash
apus status --path /path/to/project
apus status --path /path/to/project --target MyApp
apus status --json --path /path/to/project
```

### `apus remove`

Reverses `apus init`.

```bash
apus remove --path /path/to/project
apus remove --path /path/to/project --target MyApp
apus remove --path /path/to/project --dry-run
apus remove --path /path/to/project --dry-run --json
```

## Connecting Your MCP Client

Once the app is running in the simulator, point your MCP-capable client at the URL printed by `apus`.

Default URL:

```text
http://localhost:9847/mcp
```

Start here:

- [MCP client integration guide](./docs/mcp-clients.md)
- [Troubleshooting](./docs/troubleshooting.md)

## Support Model

Start with the support matrix before trying `apus init` on a new repo shape:

- [Support matrix](./docs/support-matrix.md)
- [Troubleshooting](./docs/troubleshooting.md)

The current validation corpus lives in [`fixtures/matrix.json`](./fixtures/matrix.json) and includes both synthetic fixtures and pinned open-source repos.

## How It Works

1. `apus new` or `apus init` prepares the project for Apus.
2. You build and run the app in the simulator.
3. Apus serves MCP over `http://localhost:9847/mcp` by default.
4. Your AI agent connects there to inspect logs, network, views, screenshots, and hot-reload changes.

## For Maintainers

Contributor and release workflow docs live in [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

Apache 2.0
