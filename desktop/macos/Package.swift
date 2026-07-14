// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "TeamPulse",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "TeamPulse", targets: ["TeamPulse"])
    ],
    dependencies: [],
    targets: [
        .executableTarget(name: "TeamPulse", dependencies: [])
    ]
)
