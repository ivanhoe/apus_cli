import SwiftUI

struct SharedContentView: View {
    let title: String
    let subtitle: String

    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: "square.stack.3d.up.fill")
                .font(.system(size: 48))
                .foregroundStyle(.tint)

            Text(title)
                .font(.title.bold())

            Text(subtitle)
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
        }
        .padding(24)
    }
}

#Preview {
    SharedContentView(
        title: "FixtureMultiTargetApp",
        subtitle: "Synthetic fixture with two app targets."
    )
}
