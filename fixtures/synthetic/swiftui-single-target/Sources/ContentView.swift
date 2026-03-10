import SwiftUI

struct ContentView: View {
    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: "hammer.circle.fill")
                .font(.system(size: 48))
                .foregroundStyle(.tint)

            Text("FixtureSwiftUISingleTarget")
                .font(.title.bold())

            Text("Baseline synthetic fixture for apus init/remove/status.")
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
