import SwiftUI
import WidgetKit

@main
struct FixtureWidgets: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: "FixtureWidgets", provider: Provider()) { entry in
            WidgetEntryView(entry: entry)
        }
        .configurationDisplayName("FixtureWidgets")
        .description("Synthetic widget target used for target selection coverage.")
    }
}

struct Provider: TimelineProvider {
    func placeholder(in context: Context) -> Entry {
        Entry(date: .now)
    }

    func getSnapshot(in context: Context, completion: @escaping (Entry) -> Void) {
        completion(Entry(date: .now))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<Entry>) -> Void) {
        let entry = Entry(date: .now)
        completion(Timeline(entries: [entry], policy: .never))
    }
}

struct Entry: TimelineEntry {
    let date: Date
}

struct WidgetEntryView: View {
    var entry: Entry

    var body: some View {
        Text(entry.date.formatted())
            .padding()
    }
}
