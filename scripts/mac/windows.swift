// Список окон на экране в JSON для scripts/mac (029 design §3.3): id, pid, владелец, имя, слой,
// рамка в точках (верх-лево главного экрана). Порядок — спереди назад, как отдаёт CGWindowList.
// Сборка один раз: swiftc -O windows.swift -o windows; запасной путь — swift windows.swift.
// Имя окна без права записи экрана пустое, владелец и рамка есть всегда — по ним и ищем.
import CoreGraphics
import Foundation

func num(_ v: Any?) -> Double { (v as? NSNumber)?.doubleValue ?? 0 }

let opts: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
let raw = CGWindowListCopyWindowInfo(opts, kCGNullWindowID) as? [[String: Any]] ?? []

var wins: [[String: Any]] = []
for w in raw {
    let b = w[kCGWindowBounds as String] as? [String: Any] ?? [:]
    wins.append([
        "id": Int(num(w[kCGWindowNumber as String])),
        "pid": Int(num(w[kCGWindowOwnerPID as String])),
        "owner": w[kCGWindowOwnerName as String] as? String ?? "",
        "name": w[kCGWindowName as String] as? String ?? "",
        "layer": Int(num(w[kCGWindowLayer as String])),
        "alpha": num(w[kCGWindowAlpha as String]),
        "rect": [num(b["X"]), num(b["Y"]), num(b["Width"]), num(b["Height"])],
    ])
}

// Масштаб Retina: пиксели режима экрана / точки. Снимок screencapture — в пикселях.
let disp = CGMainDisplayID()
let bounds = CGDisplayBounds(disp)
var scale = 1.0
if let mode = CGDisplayCopyDisplayMode(disp), bounds.width > 0 {
    scale = Double(mode.pixelWidth) / Double(bounds.width)
}

let doc: [String: Any] = [
    "screen": [Double(bounds.width), Double(bounds.height)],
    "scale": scale,
    "windows": wins,
]
let data = try JSONSerialization.data(withJSONObject: doc, options: [.sortedKeys])
FileHandle.standardOutput.write(data)
FileHandle.standardOutput.write("\n".data(using: .utf8)!)
