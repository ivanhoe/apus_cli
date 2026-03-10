import SwiftUI

@main
struct FixtureSecondaryApp: App {
    var body: some Scene {
        WindowGroup {
            SharedContentView(
                title: "FixtureSecondaryApp",
                subtitle: "Secondary app target that should make target auto-selection ambiguous."
            )
        }
    }
}
