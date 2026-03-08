# cloude.md — Working Memory

Date: 2026-03-04
Repo: `github.com/ivanhoe/apus_cli`

## What happened

- `apus init` was failing on a real app (`IceCubesApp`) at package resolution (`xcodebuild -resolvePackageDependencies`, exit `74`).
- The unresolved dependency was consistently Apus (`https://github.com/ivanhoe/apus`).
- Project detection also showed occasional `xcodebuild -list` failures (`exit 74`).

## What was changed in apus_cli

- Release/version fix:
  - `cmd.version` is now inject-able via ldflags (`var` instead of `const`).
  - Release workflow now points to module path `github.com/ivanhoe/apus_cli/cmd.version`.
- `apus init` reliability fixes:
  - package resolution failure returns non-zero exit code.
  - preflight checks added.
  - best-effort backup/restore flow added.
- Detection improvements:
  - target-aware `@main` entrypoint selection (avoid widget/extension mis-detection).
- PBX idempotency hardening:
  - ensures Apus package wiring exists even when URL already exists in `project.pbxproj`.
- New/updated tests:
  - xcode detect tests
  - pbxproj wiring/idempotency tests
  - preflight tests
  - scaffold tests

## Current local status

- `go test ./...` -> passing
- `go vet ./...` -> passing
- `/usr/local/bin/apus` rebuilt from this repo

## Open review notes

- Rollback wording can claim more than restored scope.
- Backup folder timestamp can collide on repeated runs within the same second.
- Step 1 rollback error is swallowed.
