# Troubleshooting

This page covers the most common ways `apus` can fail and the shortest path out of them.

## `xcodegen` is not available in `PATH`

Symptom:

```text
xcodegen not available
```

Why it happens:

- `apus new` depends on `xcodegen` to materialize the generated project

Fix:

```bash
brew install xcodegen
apus doctor
```

## No iPhone simulator runtime is available

Symptom:

```text
no available iPhone simulator was found
```

Why it happens:

- Xcode is installed, but no iPhone simulator runtime is available locally

Fix:

1. Open Xcode.
2. Go to `Settings > Platforms`.
3. Install at least one iOS Simulator runtime.
4. Re-run:

```bash
apus doctor
```

## Project detection fails or the wrong target is ambiguous

Symptom:

- `project detection failed`
- `multiple app targets found`
- `doctor` reports a target-selection failure

Why it happens:

- the repo has more than one app target
- the default target heuristic is intentionally refusing to guess

Fix:

```bash
apus doctor --path /path/to/project --target MyApp
apus init --path /path/to/project --target MyApp --dry-run
```

If you are unsure about the target name, inspect the app targets in Xcode first and then pass the exact app target.

## `doctor` reports `risky` because no Swift entry point was detected

Symptom:

```text
project:entrypoint
no Swift entry point was detected for the selected target
```

Why it happens:

- the project layout is valid enough to inspect, but `apus` could not find a safe place to inject `Apus.shared.start()`

What to do:

1. Run:

```bash
apus init --path /path/to/project --target MyApp --dry-run
```

2. If the only warning is the entry point, you can still add the package dependency with `apus init`.
3. Then add Apus manually in your startup path:
   - SwiftUI: `App.init()`
   - UIKit: `application(_:didFinishLaunchingWithOptions:)`

The manual pieces are:

- `import Apus`
- `Apus.shared.start(interceptNetwork: true)`

## Package resolution fails

Symptom:

```text
package resolution failed
```

Why it happens:

- Xcode could not resolve the Swift Package graph after `project.pbxproj` changed
- this is usually an environment, network, or package-state problem rather than a parsing problem

What to try:

```bash
apus init --path /path/to/project --dry-run
xcodebuild -resolvePackageDependencies -project /path/to/project/MyApp.xcodeproj
```

If you are developing Apus locally instead of using the GitHub package:

```bash
apus init --path /path/to/project --package-path /path/to/apus
```

If Xcode still cannot resolve packages, open the project in Xcode and resolve the packages there to get the full IDE error surface.

## `apus new` launches the app but MCP never becomes ready

Symptom:

```text
MCP health check failed
```

Why it happens:

- the app launched, but nothing answered at the expected MCP URL in time
- another running simulator app may already be using the same port

What to try:

```bash
apus new MyApp
apus new MyApp --port 9999
```

Also check:

- the app is still running in the simulator
- no other simulator app is bound to the same localhost port
- the printed MCP URL matches the port your client is trying to use

## `apus remove` says there is nothing to remove

Symptom:

```text
Apus is not integrated in this project — nothing to remove.
```

Why it happens:

- the project does not currently contain the Apus package dependency, Swift injection, or an Apus-managed `AGENTS.md`

Verify with:

```bash
apus status --path /path/to/project
apus status --json --path /path/to/project
```

## Useful inspection commands

```bash
apus doctor --path /path/to/project --json
apus init --path /path/to/project --target MyApp --dry-run
apus status --path /path/to/project --json
apus remove --path /path/to/project --dry-run --json
```
