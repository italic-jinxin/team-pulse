import SwiftUI
import AppKit

@main struct TeamPulseApp: App {
    @StateObject private var server = ServerManager()
    var body: some Scene {
        MenuBarExtra("TeamPulse", systemImage: server.running ? "waveform.path.ecg" : "exclamationmark.triangle") {
            Button("Open Dashboard") { server.openDashboard() }.disabled(!server.running)
            Text("Server Status: \(server.running ? "Running" : "Stopped")")
            Divider()
            Button("Restart Server") { server.restart() }
            Button("Open Data Directory") { server.openDataDirectory() }
            Divider()
            Button("Quit TeamPulse") { server.stop(); NSApplication.shared.terminate(nil) }
        }
    }
}

@MainActor final class ServerManager: ObservableObject {
    @Published var running=false
    private var process: Process?
    private var url=URL(string:"http://127.0.0.1:19421")!
    init(){start()}
    func start(){
        guard process == nil else{return}
        let bundled=Bundle.main.bundleURL.appendingPathComponent("Contents/MacOS/teampulse-server")
        let development=URL(fileURLWithPath:FileManager.default.currentDirectoryPath).appendingPathComponent("build/teampulse-server")
        let binary=FileManager.default.isExecutableFile(atPath:bundled.path) ? bundled : development
        guard FileManager.default.isExecutableFile(atPath:binary.path) else{return}
        let p=Process(),pipe=Pipe();p.executableURL=binary;p.standardOutput=pipe;p.standardError=FileHandle.standardError
        pipe.fileHandleForReading.readabilityHandler={ [weak self] handle in
            guard let line=String(data:handle.availableData,encoding:.utf8),let data=line.data(using:.utf8),let json=try? JSONSerialization.jsonObject(with:data) as? [String:String],let raw=json["url"],let u=URL(string:raw) else{return}
            Task{@MainActor in self?.url=u;self?.running=true;self?.openDashboard()}
        }
        p.terminationHandler={ [weak self] _ in Task{@MainActor in self?.process=nil;self?.running=false} }
        do{try p.run();process=p}catch{running=false}
    }
    func stop(){process?.terminate();process=nil;running=false}
    func restart(){stop();DispatchQueue.main.asyncAfter(deadline:.now()+0.5){self.start()}}
    func openDashboard(){NSWorkspace.shared.open(url)}
    func openDataDirectory(){let home=FileManager.default.homeDirectoryForCurrentUser;NSWorkspace.shared.open(home.appendingPathComponent("Library/Application Support/TeamPulse"))}
}
