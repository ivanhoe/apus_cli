import SwiftUI

@main
struct FixtureWidgetHostApp: App {
    var body: some Scene {
        WindowGroup {
            SharedWidgetContentView(
                title: "FixtureWidgetHostApp",
                subtitle: "Host app target that should be selected explicitly."
            )
        }
    }
}
