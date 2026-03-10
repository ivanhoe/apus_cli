// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "FixtureSupportKit",
    platforms: [
        .iOS(.v16),
    ],
    products: [
        .library(
            name: "FixtureSupportKit",
            targets: ["FixtureSupportKit"]
        ),
    ],
    targets: [
        .target(
            name: "FixtureSupportKit"
        ),
    ]
)
