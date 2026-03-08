# AGENTS.md — apus_cli

## What is this repo

Go CLI that eliminates onboarding friction for [Apus](https://github.com/ivanhoe/apus) — the runtime MCP debug server for iOS apps.

```bash
apus new MyApp   # scaffold + build + launch in one command
apus init        # add Apus to an existing Xcode project
```

## Repository layout

```
main.go
cmd/
  root.go          # cobra root, --version
  new.go           # apus new <AppName>
  init.go          # apus init
internal/
  terminal/ui.go   # spinner, step printer, color output
  scaffold/
    scaffold.go    # go:embed template renderer
    templates/     # *.tmpl files (project.yml, App.swift, ContentView.swift, AGENTS.md)
  simulator/
    simulator.go   # xcrun simctl wrapper
  builder/
    builder.go     # xcodegen + xcodebuild wrappers
  xcode/
    detect.go      # find .xcodeproj, pick target, find @main file
    inject.go      # add import Apus + Apus.shared.start() to Swift entry
    pbxproj.go     # pbxproj surgery: insert SPM dep in 5 sections
scripts/
  install.sh       # curl | bash installer
.github/workflows/
  release.yml      # arm64 + amd64 matrix, publishes on v* tags
```

## Build & Run

```bash
go build -o apus .       # build binary
go run . --help          # run without installing
go test ./...            # run tests
go vet ./...             # static analysis
```

## Key design decisions

- **Go**: single static binary, no runtime deps, easy Homebrew distribution
- **cobra**: standard Go CLI framework — subcommands, flags, --version built-in
- **go:embed**: Swift templates baked into the binary (no external template files needed)
- **pbxproj surgery** (not XcodeGen for init): regex + string insertion into existing file; 5 insertion points with `crypto/rand` UUIDs; idempotent via `apusRepoURL` presence check
- **xcodegen** (for new): avoids generating `.pbxproj` from scratch (15+ interlinked UUIDs); YAML spec is trivial to template

## pbxproj surgery details (`internal/xcode/pbxproj.go`)

Inserts 5 objects when running `apus init`:
1. `XCRemoteSwiftPackageReference` — repo URL + branch requirement (`main`)
2. `XCSwiftPackageProductDependency` — links product to the reference
3. `PBXBuildFile` — entry in the frameworks build phase
4. `packageReferences` in `PBXProject` — creates key if missing
5. Target's `packageProductDependencies` + `PBXFrameworksBuildPhase` files list

UUID format: 24-char uppercase hex (`crypto/rand`).
Idempotency check: if `https://github.com/ivanhoe/apus` already appears, it no-op; if it detects legacy `minimumVersion = 0.3.0`, it migrates to `branch = main`.

## Swift injection details (`internal/xcode/inject.go`)

1. Finds `@main` file via `filepath.Walk`
2. Idempotency: returns immediately if `import Apus` already present
3. Adds `#if DEBUG\nimport Apus\n#endif` after last `import` statement
4. If `init()` exists → inserts `Apus.shared.start(interceptNetwork: true)` as first line inside `#if DEBUG` block
5. If no `init()` → synthesizes one before `var body: some Scene {`

## Distribution

```bash
# Homebrew (once tap is published)
brew tap ivanhoe/apus
brew install apus

# curl installer
curl -fsSL https://raw.githubusercontent.com/ivanhoe/apus_cli/main/scripts/install.sh | bash

# Release: tag triggers GitHub Actions matrix build
git tag v0.x.0 && git push --tags
```

## Apus MCP reference (for context)

The CLI integrates with Apus which runs at `http://localhost:9847/mcp` inside the target iOS app.
Key tools: `get_logs`, `get_screenshot`, `get_view_hierarchy`, `ui_interact`, `get_network_history`, `hot_reload`.

## Working Memory (2026-03-04)

### Context from this session

- Main incident: `apus init` repeatedly failed at step `[2/4]` on an external app (`IceCubesApp`) with `xcodebuild -resolvePackageDependencies` exit status `74` and unresolved package graph for `https://github.com/ivanhoe/apus`.
- Symptoms included intermittent `xcodebuild -list: exit status 74` during project detection and repeated logs with `DVTBuildVersion` warnings.
- Repository was renamed from `apus-cli` to `apus_cli` and synced to `https://github.com/ivanhoe/apus_cli`.

### Fixes implemented in this repo

- Release/version wiring:
  - `cmd/root.go`: `version` changed from `const` to `var` for `-ldflags -X` injection.
  - `.github/workflows/release.yml`: ldflags module path updated to `github.com/ivanhoe/apus_cli/cmd.version`.
- `apus init` behavior:
  - If package resolution fails, command now returns non-zero (no false success).
  - Best-effort messaging + preflight checks + backup/restore path were added.
- Entry-point detection:
  - `internal/xcode/detect.go` now chooses `@main` file using target-aware scoring (prefers app target over widget/extension candidates).
- PBX project idempotency:
  - `internal/xcode/pbxproj.go` now ensures full Apus wiring even when repo URL already exists:
    - package reference link
    - package product dependency link
    - frameworks build file link
  - Handles legacy and local-reference normalization paths more robustly.
- Test coverage additions:
  - target-aware entrypoint tests (`detect_test.go`)
  - idempotent/wiring tests for pbxproj (`pbxproj_test.go`)
  - preflight/scaffold tests were added for new command behavior.

### Current status

- Validation run in this workspace:
  - `go test ./...` passes
  - `go vet ./...` passes
- Global binary `/usr/local/bin/apus` was rebuilt and reinstalled from this repo.

### Known risks / review notes

- Rollback messaging in `cmd/init.go` can overstate what is restored (backup scope is limited to selected files).
- Backup directory naming uses second-level granularity and can collide on very fast repeated runs.
- Step 1 rollback error is currently ignored (`_ = backup.restore()`).
