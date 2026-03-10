import SwiftUI

struct SharedWidgetContentView: View {
    let title: String
    let subtitle: String

    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: "rectangle.3.group.bubble.left.fill")
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
    SharedWidgetContentView(
        title: "FixtureWidgetHostApp",
        subtitle: "Synthetic host app with a widget target."
    )
}
