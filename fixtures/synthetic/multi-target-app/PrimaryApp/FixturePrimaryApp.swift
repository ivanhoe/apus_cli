import SwiftUI

@main
struct FixturePrimaryApp: App {
    var body: some Scene {
        WindowGroup {
            SharedContentView(
                title: "FixturePrimaryApp",
                subtitle: "Primary app target used for explicit --target validation."
            )
        }
    }
}
