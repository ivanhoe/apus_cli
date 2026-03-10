import SwiftUI
import FixtureSupportKit

struct ContentView: View {
    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: "shippingbox.circle.fill")
                .font(.system(size: 48))
                .foregroundStyle(.tint)

            Text("FixtureExistingSPMDependencies")
                .font(.title.bold())

            Text(FixtureSupportKit.bannerText)
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
        }
        .padding(24)
    }
}

#Preview {
    ContentView()
}
