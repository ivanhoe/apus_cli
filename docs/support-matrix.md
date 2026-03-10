# Support Matrix

`apus` is designed to do one of two things:

- integrate cleanly
- refuse cleanly before mutating files

Use `apus doctor` first. Its classification is the contract.

## Classification meanings

### `supported`

The current project shape is on the validated happy path.

Typical workflow:

```bash
apus doctor --path /path/to/project
apus init --path /path/to/project --dry-run
apus init --path /path/to/project
```

### `risky`

`apus` can probably add the package dependency safely, but some integration steps may still require manual work.

Typical examples:

- no Swift entry point was detected for the selected target
- project layout is unusual but still partially inspectable

Recommended workflow:

```bash
apus doctor --path /path/to/project --target MyApp
apus init --path /path/to/project --target MyApp --dry-run
```

Only proceed if the dry-run matches what you expect.

### `unsupported`

`apus` should stop before mutating the project.

Typical examples:

- no supported `.xcodeproj` could be detected
- the target cannot be chosen safely
- required local tooling is missing

In this state, do not run `apus init` until `doctor` is clean.

## Project shapes

| Project shape | Status | Notes |
| --- | --- | --- |
| Single-target SwiftUI app | supported | Standard happy path for `apus init` |
| UIKit / `AppDelegate` app | supported | Covered by synthetic fixture validation |
| Project with existing Swift Package dependencies | supported | `project.pbxproj` wiring is validated against an existing package graph |
| Multi-target app repo | supported with `--target` | Pass the app target explicitly |
| App plus widget, App Clip, or companion targets | supported with `--target` | Choose the iOS app target, not the extension target |
| Project detected but no Swift entry point found | risky | Apus can usually add the dependency, but you may need to add `Apus.shared.start()` manually |
| Non-standard layout where the app target cannot be detected safely | unsupported | `apus` should fail before writing files |

## Real-world validation

The current fixture matrix includes:

- synthetic fixtures for SwiftUI, UIKit, multi-target, widget, existing SPM, and unsupported layouts
- pinned open-source validation against:
  - `nalexn/clean-architecture-swiftui`
  - `apple-sample-code/FrutaBuildingAFeatureRichAppWithSwiftUI`

Source of truth:

- [`fixtures/matrix.json`](../fixtures/matrix.json)
- [`fixtures/README.md`](../fixtures/README.md)

## Best practices

- run `apus doctor` before `apus init`
- use `--target` anytime the repo has more than one app target
- use `--dry-run` before mutating a repo you care about
- use `apus status` after integration to confirm dependency, Swift injection, and `AGENTS.md`
