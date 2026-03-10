# Fixture Matrix

This directory tracks the validation corpus for `apus` against real-world project shapes.

The source of truth is [`matrix.json`](./matrix.json). Each fixture is one project class we want
to validate repeatedly, not just a one-off repo name.

## Stages

- `planned`: coverage we know we need, but have not pinned or built yet.
- `ready`: fixture has enough location metadata to be executed by a harness.

## Outcomes

- `supported`: `status`, `init`, and `remove` should succeed without extra flags.
- `supported-with-target`: success requires an explicit `--target`.
- `unsupported-cleanly`: the CLI should fail before writing any files.

## Workflow

1. Add or update a fixture entry in `matrix.json`.
2. Promote it from `planned` to `ready` only after pinning the repo/path and expected behavior.
3. Turn every externally discovered regression into a fixture entry or a synthetic regression test.

Ready synthetic fixtures live under `fixtures/synthetic/<id>/` and should be fully self-contained.
For Xcode-based fixtures, prefer `xcodegen` inputs over checked-in `.xcodeproj` bundles so diffs stay reviewable.
Ready external fixtures are pinned by repo URL + SHA and are cloned into a temp workspace by `fixturerunner`.

## Local Commands

```bash
go run ./tools/fixturematrix validate
go run ./tools/fixturematrix list
go run ./tools/fixturematrix plan
```
